//===- Hello.cpp - Example code from "Writing an LLVM Pass" ---------------===//
//
//                     The LLVM Compiler Infrastructure
//
// This file is distributed under the University of Illinois Open Source
// License. See LICENSE.TXT for details.
//
//===----------------------------------------------------------------------===//
//
// 
// 
//
//===----------------------------------------------------------------------===//

#define DEBUG_TYPE "hello"
#include "llvm/Pass.h"
#include "llvm/PassManager.h"
#include "llvm/Transforms/IPO/PassManagerBuilder.h"

#include "llvm/IR/Module.h"
#include "llvm/IR/Function.h"

#include "llvm/IR/IntrinsicInst.h"
#include "llvm/IR/Type.h"

// TODO(pwaller): it has been renamed to DataLayout
// #include "llvm/Target/TargetData.h"
// #include "llvm/Target/TargetMachine.h"

#include "llvm/Support/raw_ostream.h"

#include "llvm/Config/config.h"
#if LLVM_VERSION_MAJOR < 3
  #error "Unsupported LLVM version"
#elif LLVM_VERSION_MAJOR == 3 && LLVM_VERSION_MINOR < 2
  #include "llvm/Support/IRBuilder.h"
#else
  #include "llvm/IR/IRBuilder.h"
#endif

#include "llvm/Transforms/Utils/ModuleUtils.h"

#include <vector>
#include <cxxabi.h>

using namespace llvm;

static const int   kAsanCtorAndCtorPriority = 1;
static const char *kAsanModuleCtorName = "mema.module_ctor";
static const char *kAsanInitName = "__mema_initialize";

namespace {

  // Validate the result of Module::getOrInsertFunction called for an interface
  // function of AddressSanitizer. If the instrumented module defines a function
  // with the same name, their prototypes must match, otherwise
  // getOrInsertFunction returns a bitcast.
  Function *checkInterfaceFunction(Constant *FuncOrBitcast) {
    if (isa<Function>(FuncOrBitcast)) return cast<Function>(FuncOrBitcast);
    FuncOrBitcast->dump();
    report_fatal_error("trying to redefine an AddressSanitizer "
                       "interface function");
  }

  struct MemaPass : public ModulePass {
    static char ID;
    MemaPass() : ModulePass(ID) {}
    //virtual const char *getPassName() const { return "Hello"; }

    LLVMContext * C;
    DataLayout * TD;
    int LongSize;
    Type * IntptrTy;
    
    Function* MemAccessCallback;
    
    Function *AsanCtorFunction;
    Function *AsanInitFunction;
    Instruction *CtorInsertBefore;

    virtual bool runOnModule(Module &M) {
      TD = getAnalysisIfAvailable<DataLayout>();
      if (!TD) return false;
      C = &(M.getContext());
      LongSize = TD->getPointerSizeInBits();
      IntptrTy = Type::getIntNTy(*C, LongSize);
      
      AsanCtorFunction = Function::Create(
      FunctionType::get(Type::getVoidTy(*C), false),
      GlobalValue::InternalLinkage, kAsanModuleCtorName, &M);
      BasicBlock *AsanCtorBB = BasicBlock::Create(*C, "", AsanCtorFunction);
      CtorInsertBefore = ReturnInst::Create(*C, AsanCtorBB);

      // call __asan_init in the module ctor.
      IRBuilder<> IRB(CtorInsertBefore);
      AsanInitFunction = checkInterfaceFunction(
          M.getOrInsertFunction(kAsanInitName, IRB.getVoidTy(), NULL));
      AsanInitFunction->setLinkage(Function::ExternalLinkage);
      IRB.CreateCall(AsanInitFunction);
      
      MemAccessCallback = cast<Function>(
        M.getOrInsertFunction("__mema_access", Type::getVoidTy(*C),
                              IntptrTy, Type::getInt8Ty(*C), 
                              Type::getInt1Ty(*C), NULL));
      
      bool Res = false;
      for (Module::iterator F = M.begin(), E = M.end(); F != E; ++F) {
        if (F->isDeclaration()) continue;
        Res |= handleFunction(M, *F);
      }
      
      appendToGlobalCtors(M, AsanCtorFunction, kAsanCtorAndCtorPriority);
      
      return false;
    }
    
    static Value *isInterestingMemoryAccess(Instruction *I, bool *IsWrite) {
      if (LoadInst *LI = dyn_cast<LoadInst>(I)) {
        *IsWrite = false;
        return LI->getPointerOperand();
      }
      if (StoreInst *SI = dyn_cast<StoreInst>(I)) {
        *IsWrite = true;
        return SI->getPointerOperand();
      }
      if (AtomicRMWInst *RMW = dyn_cast<AtomicRMWInst>(I)) {
        *IsWrite = true;
        return RMW->getPointerOperand();
      }
      if (AtomicCmpXchgInst *XCHG = dyn_cast<AtomicCmpXchgInst>(I)) {
        *IsWrite = true;
        return XCHG->getPointerOperand();
      }
      return NULL;
    }
    
    std::vector<Function*> functions;

    bool handleFunction(Module &M, Function &F) {
      if (&F == AsanCtorFunction) return false;
    
      // errs() << "Handling function: ";
      // errs().write_escaped(F.getName()) << '\n';
      
      // Instrument function calls
      IRBuilder<> myIRB(F.begin()->getFirstNonPHI());
      
      Function * func_entry = cast<Function>(
        M.getOrInsertFunction("__mema_function_entry", myIRB.getVoidTy(), IntptrTy, NULL)),
               * func_exit  = cast<Function>(
        M.getOrInsertFunction("__mema_function_exit",  myIRB.getVoidTy(), IntptrTy, NULL));
      
      Value* funcptr = myIRB.CreatePointerCast(&F, IntptrTy);
      // Instrument function entry
      myIRB.CreateCall(func_entry, funcptr);
      
      /// Instrument function returns
      for (Function::iterator FI = F.begin(), FE = F.end(); FI != FE; ++FI) {
        for (BasicBlock::iterator BI = FI->begin(), BE = FI->end(); BI != BE; ++BI) {
          if (!isa<ReturnInst>(BI)) continue;
          IRBuilder<> irb(BI);
          irb.CreateCall(func_exit, funcptr);
        }
      }
      
      bool IsWrite;
      
      for (Function::iterator FI = F.begin(), FE = F.end(); FI != FE; ++FI) {
        for (BasicBlock::iterator BI = FI->begin(), BE = FI->end(); BI != BE; ++BI) {
          //errs() << "-- " << BI->getOpcodeName() << "\n";
          //BI->dump();
          //errs() << "\n";
        
          if (Value *Addr = isInterestingMemoryAccess(BI, &IsWrite)) {
            /*
            BI->getDebugLoc().dump(*C);
            errs() << " Memory access " << (IsWrite ? "Write" : "Read") << "\n";
            BI->dump();
            errs() << "Addr: ";
            Addr->dump();
            */
            
            Type *OrigPtrTy = Addr->getType();
            Type *OrigTy = cast<PointerType>(OrigPtrTy)->getElementType();

            assert(OrigTy->isSized());
            uint32_t TypeSize = TD->getTypeStoreSizeInBits(OrigTy);
            
            IRBuilder<> IRB(BI);
            Value *AddrLong = IRB.CreatePointerCast(Addr, IntptrTy);
            IRB.CreateCall3(MemAccessCallback, AddrLong,
                            ConstantInt::get(Type::getInt8Ty(*C), TypeSize),
                            ConstantInt::get(Type::getInt1Ty(*C), IsWrite));
            
          } else if (isa<MemIntrinsic>(BI)) {
            errs() << "  !!!!!!!!! Memory intrinsic\n";
            MemIntrinsic* MI = cast<MemIntrinsic>(BI);
            //MI->getDest()->dump();
            //MI->getRawDest()->dump();
            
            IRBuilder<> IRB(BI);
            Value *AddrLong = IRB.CreatePointerCast(MI->getRawDest(), IntptrTy);
            IRB.CreateCall3(MemAccessCallback, AddrLong,
                            ConstantInt::get(Type::getInt8Ty(*C), 0),
                            ConstantInt::get(Type::getInt1Ty(*C), true));
                            
            //errs() << "-- length:\n";
            //MI->getLength()->dump();
            if (isa<MemTransferInst>(BI)) {
                //errs() << "-- memtransfer src:\n";
                MemTransferInst* MTI = cast<MemTransferInst>(MI);
                //MTI->getSource()->dump();
                // TODO: Something with the source address?
            }
          } else if (isa<CallInst>(BI)) {
            //errs() << "  Function call..\n";
            // Maybe we could instrument foreign calls?
          } else {
            continue;
          }
        }
      }
      
      //errs() << "  Here: " << __LINE__ << "\n";
      
      return true;
    }
  };
}

char MemaPass::ID = 0;
void InitMemaPass(const PassManagerBuilder &Builder, PassManagerBase &PM) {
    PM.add(new MemaPass());
}

static RegisterStandardPasses rp_noopt(PassManagerBuilder::EP_EnabledOnOptLevel0, InitMemaPass);
static RegisterStandardPasses rp_opt  (PassManagerBuilder::EP_ModuleOptimizerEarly, InitMemaPass);

