#! /usr/bin/env bash

echo " -- Fetching and installing valgrind + memadump -- "

set -e -u

P=valgrind-3.8.1

if [ ! -e ${P}.tar.bz2 ]; then
	wget http://www.valgrind.org/downloads/${P}.tar.bz2
fi

if [ -d ${P} ]; then
	# Start afresh
	rm -r ${P}
fi

tar xf ${P}.tar.bz2

pushd ${P}
cp -sr ${PWD}/../memadump .
patch < ../valgrind.patch
./autogen.sh

mkdir build
cd build
../configure --prefix=${PWD}/../install
ln -s ../*.supp .
make -j3
make -j3 install

echo
echo " -- Done installing valgrind and memadump --"
echo "    run: ${PWD}/../install/bin/valgrind --tool=memadump"
