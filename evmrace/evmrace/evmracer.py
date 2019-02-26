#!/usr/bin/python

import jinja2, json, re, os, shutil
from functools import reduce
import subprocess
import nanodurationpy as durationpy
import csv

# output paths should be mounted docker volumes
WASM_FILE_OUTPUT_PATH = "/evmwasmfiles"
RESULT_CSV_OUTPUT_PATH = "/evmraceresults"

def get_rust_bytes(hex_str):
    tmp = map(''.join, zip(*[iter(hex_str)]*2))
    tmp = map(lambda x: int(x, 16), tmp)
    tmp = map(lambda x: '{}u8'.format(x), tmp)
    tmp = reduce(lambda x, y: x+', '+y, tmp)
    return '[ '+tmp+' ]'

def bench_rust_binary(rustdir, input_name, native_exec):
    print("running rust native {}...\n{}".format(input_name, native_exec))
    # TODO: get size of native exe file
    bench_times = []
    for i in range(1,20):
        rust_process = subprocess.Popen(native_exec, cwd=rustdir, stderr=subprocess.STDOUT, stdout=subprocess.PIPE, shell=True)
        rust_process.wait(None)
        stdoutlines = [str(line, 'utf8') for line in rust_process.stdout]
        print(("").join(stdoutlines), end="")
        elapsedline = stdoutlines[0]
        elapsedmatch = re.search("Time elapsed in bench\(\) is: ([\w\.]+)", elapsedline)
        elapsed_time = durationpy.from_str(elapsedmatch[1])
        bench_times.append(elapsed_time.total_seconds())
    return bench_times

def do_rust_bench(benchname, input):
    #rustsrc = "{}/rust-code/src/bench.rs".format(os.path.abspath(benchname))
    rustsrc = "{}/rust-code".format(os.path.abspath(benchname))
    rusttemplate = "{}/src/bench.rs".format(rustsrc)

    filldir = os.path.abspath("{}/rust-code-filled".format(benchname))
    if os.path.exists(filldir):
        shutil.rmtree(filldir)
    shutil.copytree(rustsrc, filldir)

    input_len = int(len(input['input']) / 2)
    input_str = "let input: [u8; {}] = {};".format(input_len, get_rust_bytes(input['input']))
    expected_len = int(len(input['expected']) / 2)
    expected_str = "let expected: [u8; {}] = {};".format(expected_len, get_rust_bytes(input['expected']))

    with open(rusttemplate) as file_:
        template = jinja2.Template(file_.read())
        filledrust = template.render(input=input_str, expected=expected_str)

    rustfileout = "{}/src/bench.rs".format(filldir)
    with open(rustfileout, 'w') as outfile:
        outfile.write(filledrust)

    # compile rust code
    rust_native_cmd = "cargo build --release --bin {}_native".format(benchname)
    print("compiling rust native {}...\n{}".format(input['name'], rust_native_cmd))
    rust_process = subprocess.Popen(rust_native_cmd, cwd=filldir, stderr=subprocess.STDOUT, stdout=subprocess.PIPE, shell=True)
    rust_process.wait(None)
    stdoutlines = [str(line, 'utf8') for line in rust_process.stdout]
    print(("").join(stdoutlines), end="")
    # native binary is at ./target/release/sha1_native
    exec_path = "{}/target/release/{}_native".format(filldir, benchname)
    exec_size = os.path.getsize(exec_path)

    # TODO: also build with optimization turned off
    rust_wasm_cmd = "cargo build --release --lib --target wasm32-unknown-unknown"
    print("compiling rust wasm {}...\n{}".format(input['name'], rust_wasm_cmd))
    rust_process = subprocess.Popen(rust_wasm_cmd, cwd=filldir, stderr=subprocess.STDOUT, stdout=subprocess.PIPE, shell=True)
    rust_process.wait(None)
    stdoutlines = [str(line, 'utf8') for line in rust_process.stdout]
    print(("").join(stdoutlines), end="")
    # wasm is at ./target/wasm32-unknown-unkown/release/sha1_wasm.wasm
    wasmbin = "{}/target/wasm32-unknown-unknown/release/{}_wasm.wasm".format(filldir, benchname)
    wasmdir = os.path.abspath(WASM_FILE_OUTPUT_PATH)
    wasmoutfile = "{}/{}.wasm".format(wasmdir, input['name'])
    if not os.path.exists(wasmdir):
        os.mkdir(wasmdir)
    shutil.copy(wasmbin, wasmoutfile)
    
    # TODO: get cargo build compiler time and report along with exec time.

    # run rust binary
    native_times = bench_rust_binary(filldir, input['name'], "./target/release/{}_native".format(benchname))
    return { 'bench_times': native_times, 'exec_size': exec_size }

def do_go_bench(benchname, input):
    #COPY ./sha1_test.go /go-ethereum/core/vm/runtime/sha_test.go
    #RUN cd /go-ethereum/core/vm/runtime && go test -bench BenchmarkSHA1 -benchtime 5s
    destdir = "/go-ethereum/core/vm/runtime/"
    # first letter must be capitalized or go bench command doesnt work
    goBenchName = benchname[:1].upper() + benchname[1:]
    go_cmd = "go test -bench Benchmark{} -benchtime 5s".format(goBenchName)
    gofile = "{}_test.go".format(benchname)

    # fill go template
    with open("./" + benchname + "/" + gofile) as file_:
        template = jinja2.Template(file_.read())
        filledgo = template.render(input=input['input'], expected=input['expected'])

    gofileout = "{}/{}_filled_test.go".format(os.path.abspath(benchname), benchname)
    with open(gofileout, 'w') as outfile:
        outfile.write(filledgo)

    # copy benchmark file
    shutil.copy(gofileout, destdir)

    # run go command
    print("running go benchmark {}...\n{}".format(input['name'], go_cmd))
    go_process = subprocess.Popen(go_cmd, cwd=destdir, stderr=subprocess.STDOUT, stdout=subprocess.PIPE, shell=True)
    go_process.wait(None)
    stdoutlines = [str(line, 'utf8') for line in go_process.stdout]
    print(("").join(stdoutlines), end="")
    """
    running benchmark sha1-10808-bits...
    gasUsed: 1543776
    goos: linux
    goarch: amd64
    pkg: github.com/ethereum/go-ethereum/core/vm/runtime
    BenchmarkSha1-4         gasUsed: 1543776
    gasUsed: 1543776
         200          44914864 ns/op
    PASS
    ok      github.com/ethereum/go-ethereum/core/vm/runtime 13.472s
    """
    nsperopline = stdoutlines[-3]
    gasline = stdoutlines[-4]
    nsperop_match = re.search("\d+\s+(\d+) ns/op", nsperopline)
    ns_time = durationpy.from_str("{}ns".format(nsperop_match[1]))
    gas_match = re.search("gasUsed: (\d+)", gasline)
    gasused = gas_match[1]
    return {'gasUsed': gasused, 'time': ns_time.total_seconds()}


def saveResults(native_benchmarks, evm_benchmarks):
    native_file = "{}/native_benchmarks.csv".format(RESULT_CSV_OUTPUT_PATH)
    with open(native_file, 'w', newline='') as bench_result_file:
        fieldnames = ['test_name', 'elapsed_times', 'native_file_size']
        writer = csv.DictWriter(bench_result_file, fieldnames=fieldnames)
        writer.writeheader()
        for test_name, test_results in native_benchmarks.items():
            bench_times = [str(t) for t in test_results['bench_times']]
            times_str = ", ".join(bench_times)
            writer.writerow({"test_name" : test_name, "elapsed_times" : times_str, "native_file_size" : test_results['exec_size']})

    evm_file = "{}/evm_benchmarks.csv".format(RESULT_CSV_OUTPUT_PATH)
    with open(evm_file, 'w', newline='') as bench_result_file:
        fieldnames = ['test_name', 'elapsed_time', 'gas_used']
        writer = csv.DictWriter(bench_result_file, fieldnames=fieldnames)
        writer.writeheader()
        for test_name, test_results in evm_benchmarks.items():
            writer.writerow({"test_name" : test_name, "elapsed_time" : test_results['time'], "gas_used" : test_results['gasUsed']})


def main():
    benchdirs = [dI for dI in os.listdir('./') if os.path.isdir(os.path.join('./',dI))]
    native_benchmarks = {}
    evm_benchmarks = {}
    for benchname in benchdirs:
        if benchname == "__pycache__":
            continue
        with open("{}/{}-inputs.json".format(benchname, benchname)) as f:
            bench_inputs = json.load(f)
            for input in bench_inputs:
                #input['name'], input['input'], input['expected']
                #do_go_bench(benchname, input)
                native_input_times = do_rust_bench(benchname, input)
                native_benchmarks[input['name']] = native_input_times
                go_evm_times = do_go_bench(benchname, input)
                evm_benchmarks[input['name']] = go_evm_times

    print("got native_benchmarks:", native_benchmarks)
    print("got evm_benchmarks:", evm_benchmarks)
    saveResults(native_benchmarks, evm_benchmarks)

if __name__ == "__main__":
    main()