# gonexus

`gonexus` is a lightweight tool and library designed for inspecting and reading [NeXus](https://www.nexusformat.org/) data files (HDF5-based) in both Go and Python.

---

## Prerequisites

* **Go**: 1.20 or later
* **Python**: 3.8 or later
* **Sample Data**: A valid `.nxs` file for testing

---

## Quickstart

### 1. Download Sample NeXus Data

You can fetch a sample NeXus dataset from the official [NeXus Example Data](https://github.com/nexusformat/exampledata) repository:

```bash
curl -ksLO [https://raw.githubusercontent.com/nexusformat/exampledata/master/Soleil/hdf5/file_1.nxs](https://raw.githubusercontent.com/nexusformat/exampledata/master/Soleil/hdf5/file_1.nxs)

```

---

### 2. Go Usage

#### Build and Run

To compile and run the Go executable against the sample file:

```bash
# Build the binary
go build -o nexus

# Inspect the NeXus file
./nexus -fin file_1.nxs

```

### 3. Python Usage

#### Virtual Environment Setup

Create a virtual environment and install the required HDF5 dependencies:

```bash
# Create and activate virtual environment
python3 -m venv venv
source venv/bin/activate

# Install requirements
pip install h5py

```

#### Run Script

Execute the Python inspector script:

```bash
python nexus.py --fin file_1.nxs

```

---

## Command Line Flags

| Flag | Description |
| --- | --- |
| `-fin`, `--fin` | Path to the input NeXus (`.nxs` / `.h5`) file |

---

## License

Distributed under the MIT License. See `LICENSE` for more information.
