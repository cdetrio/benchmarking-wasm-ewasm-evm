FROM ubuntu:18.04

# System deps
RUN apt-get update
RUN apt-get install -y software-properties-common git sudo build-essential wget curl nano \
    autoconf automake cmake libtool llvm-6.0 make ninja-build unzip zlib1g-dev texinfo


# install python 3.7
RUN apt-add-repository ppa:deadsnakes/ppa && apt-get update
RUN apt-get install -y python3.7
RUN wget https://bootstrap.pypa.io/get-pip.py && python3.7 get-pip.py
# linking /usr/bin/python3 to /usr/bin/python3.7 will break add-apt-reposity
# RUN update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.7 1


# Install Go 1.11 (update 2019-03-16: now installs go 1.12)
RUN add-apt-repository ppa:longsleep/golang-backports && apt-get update && apt-get install -y golang-go

# Install Clang 8 (needed for life -polymerase)
RUN wget -O - https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add - && \
    apt-add-repository "deb http://apt.llvm.org/bionic/ llvm-toolchain-bionic-8 main" && \
    apt-get update && apt-get install -y clang-8 lldb-8 lld-8

RUN ln -s /usr/bin/clang-8 /usr/bin/clang && \
    ln -s /usr/bin/clang++-8 /usr/bin/clang++


RUN apt-get clean autoclean
RUN apt-get autoremove -y


# enable go modules: https://github.com/golang/go/wiki/Modules
RUN export GO111MODULE=on

WORKDIR /engines

# install life
RUN git clone --single-branch --branch bench-times https://github.com/cdetrio/life
RUN cd life && go mod vendor
RUN cd life && go build


# install wagon
RUN git clone --single-branch --branch bench-times https://github.com/cdetrio/wagon
RUN cd wagon/cmd/wasm-run && go build


# install rust
RUN curl https://sh.rustup.rs -sSf | \
    sh -s -- --default-toolchain stable -y && . $HOME/.cargo/env
ENV PATH=/root/.cargo/bin:$PATH

RUN rustup default 1.31.0


# install wasmi
RUN git clone --single-branch --branch bench-time https://github.com/cdetrio/wasmi.git --recursive
#RUN cd wasmi && cargo build --release
RUN cd wasmi && cargo test --release


# install wavm
RUN git clone --single-branch --branch bench-compile-time https://github.com/cdetrio/WAVM
RUN mkdir wavm-build
RUN cd wavm-build && cmake -G Ninja ../WAVM -DCMAKE_BUILD_TYPE=RelWithDebInfo
RUN cd wavm-build && ninja


# install binaryen
#RUN git clone https://github.com/WebAssembly/binaryen.git
#RUN cd binaryen && cmake . && make


## wasmer superseded by wasmtime
# install wasmer
# download wasmer binary
#RUN curl https://get.wasmer.io -sSfL | sh
#RUN /bin/bash -c "source /root/.wasmer/wasmer.sh"
# build wasmer from source
# wasmer release 0.1.4 has segmentation violation with rustc 1.32.0. use 1.31.1
#RUN rustup default 1.31.1
#  0.1.4 release from December 2018 is faster than latest wasmer master (2019-2-16) 
#RUN git clone --single-branch --branch bench-release https://github.com/cdetrio/wasmer.git
#RUN cd wasmer && cargo build --release
# bug where running wasmer v0.1.4 with python `Popen(stderr=subprocess.STDOUT)`
# causes an error: `Runtime error: trap at 0x0 - illegal instruction`.
# the fix is to run Popen without the stderr flag.
#RUN rustup default nightly-2019-01-15
#RUN git clone --single-branch --branch bench-compile-time https://github.com/cdetrio/wasmer.git wasmer-master
#RUN cd wasmer-master && cargo build --release


# install wasmtime
RUN rustup default nightly-2019-01-15
RUN git clone --single-branch --branch bench-times https://github.com/cdetrio/wasmtime.git
RUN cd wasmtime && cargo build --release


# install wabt
RUN git clone --recursive --single-branch --branch bench-times https://github.com/cdetrio/wabt.git
RUN mkdir wabt/build && cd wabt/build && cmake -DCMAKE_BUILD_TYPE=Release -DBUILD_TESTS=OFF .. && make


# install python modules needed for benchmarking script
RUN pip3 install click durationpy


# install nodejs
#RUN mkdir node
RUN mkdir -p node
RUN cd node && curl -fsSLO --compressed https://nodejs.org/dist/v11.10.0/node-v11.10.0-linux-x64.tar.gz
RUN cd node && tar -xvf node-v11.10.0-linux-x64.tar.gz -C /usr/local/ --strip-components=1 --no-same-owner
RUN cd node && ln -s /usr/local/bin/node ./node
COPY node-timer.js ./node/node-timer.js

# curl -fsSLO --compressed https://nodejs.org/download/nightly/v12.0.0-nightly20190325cdb87d9548/node-v12.0.0-nightly20190325cdb87d9548-linux-x64.tar.gz


# install java
ENV JAVA_VER 8
ENV JAVA_HOME /usr/lib/jvm/java-8-oracle

RUN echo 'deb http://ppa.launchpad.net/webupd8team/java/ubuntu trusty main' >> /etc/apt/sources.list && \
    echo 'deb-src http://ppa.launchpad.net/webupd8team/java/ubuntu trusty main' >> /etc/apt/sources.list && \
    apt-key adv --keyserver keyserver.ubuntu.com --recv-keys C2518248EEA14886 && \
    apt-get update && \
    echo oracle-java${JAVA_VER}-installer shared/accepted-oracle-license-v1-1 select true | sudo /usr/bin/debconf-set-selections && \
    apt-get install -y --force-yes --no-install-recommends oracle-java${JAVA_VER}-installer oracle-java${JAVA_VER}-set-default && \
    apt-get clean && \
    rm -rf /var/cache/oracle-jdk${JAVA_VER}-installer

# install asmble
RUN wget https://github.com/cdetrio/asmble/releases/download/0.4.2-fl-bench-times/asmble-0.4.2-fl-bench-times.tar
RUN tar -xvf asmble-0.4.2-fl-bench-times.tar


# install wag
#RUN apt-get install -y libcapstone-dev
#RUN git clone --single-branch --branch runtime-design https://github.com/gballet/wag.git
#RUN cd wag && go build ./cmd/ethereum/...


## install dependencies for standalone wasm prep
RUN pip3 install jinja2 pandas

# rust wasm32 target for compiling wasm
RUN rustup target add wasm32-unknown-unknown


# copy benchmarking scripts
RUN mkdir /benchrunner
COPY project /benchrunner/project
COPY main.py /benchrunner


# copy scripts to generate standalone wasm modules
RUN mkdir /benchprep
COPY main.py /benchprep
COPY benchnativerust_prepwasm.py /benchprep
COPY nanodurationpy.py /benchprep
COPY rust-code /benchprep/rust-code
COPY inputvectors /benchprep/inputvectors
COPY benchmeteredstandalone.sh /benchprep
RUN chmod +x /benchprep/benchmeteredstandalone.sh
COPY bench_wasm_and_native.sh /benchprep
RUN chmod +x /benchprep/bench_wasm_and_native.sh


RUN mkdir -p /testresults

WORKDIR /benchprep

CMD /bin/bash

