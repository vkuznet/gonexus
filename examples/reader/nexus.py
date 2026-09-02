#!/usr/bin/env python
#-*- coding: utf-8 -*-

# system modules
import argparse
import h5py


class OptionParser():
    def __init__(self):
        "User based option parser"
        self.parser = argparse.ArgumentParser(prog='PROG')
        self.parser.add_argument("--fin", action="store",
            dest="fin", default="", help="Input file")

def main():
    "Main function"
    optmgr  = OptionParser()
    opts = optmgr.parser.parse_args()

    # Read using standard h5py (since NeXus is built on top of HDF5)
    with h5py.File(opts.fin, "r") as f:
        print("Keys in root:", list(f.keys()))

if __name__ == '__main__':
    main()

