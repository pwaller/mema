
/*--------------------------------------------------------------------*/
/*--- Memadump: Dump memory accesses                     md_main.c ---*/
/*--------------------------------------------------------------------*/

/*
   This file is part of Memadump, a Valgrind tool for dumping memory
   accesses for use in Memaviz.

   Copyright (C) 2012-2013 Peter Waller and Johannes Ebke
      p@pwaller.net johannes@ebke.org

   This program is free software; you can redistribute it and/or
   modify it under the terms of the GNU General Public License as
   published by the Free Software Foundation; either version 2 of the
   License, or (at your option) any later version.

   This program is distributed in the hope that it will be useful, but
   WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
   General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program; if not, write to the Free Software
   Foundation, Inc., 59 Temple Place, Suite 330, Boston, MA
   02111-1307, USA.

   The GNU General Public License is contained in the file COPYING.
*/

#include "pub_tool_basics.h"
#include "pub_tool_tooliface.h"

static void md_post_clo_init(void)
{
}

static
IRSB* md_instrument ( VgCallbackClosure* closure,
                      IRSB* bb,
                      VexGuestLayout* layout, 
                      VexGuestExtents* vge,
                      VexArchInfo* archinfo_host,
                      IRType gWordTy, IRType hWordTy )
{
    return bb;
}

static void md_fini(Int exitcode)
{
}

static void md_pre_clo_init(void)
{
   VG_(details_name)            ("Memadump");
   VG_(details_version)         (NULL);
   VG_(details_description)     ("Dump memory accesses suitable for Memaviz");
   VG_(details_copyright_author)(
      "Copyright (C) 2012-2013, and GNU GPL'd, by Peter Waller and Johannes Ebke.");
   VG_(details_bug_reports_to)  (VG_BUGS_TO);

   VG_(details_avg_translation_sizeB) ( 275 );

   VG_(basic_tool_funcs)        (md_post_clo_init,
                                 md_instrument,
                                 md_fini);

   /* No needs, no core events to track */
}

VG_DETERMINE_INTERFACE_VERSION(md_pre_clo_init)

/*--------------------------------------------------------------------*/
/*--- end                                                          ---*/
/*--------------------------------------------------------------------*/
