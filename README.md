<div align="center">

<img src="docs/image/header-cover.jpg" alt="icon"/>

<h1 align="center">go-callflow-vis</h1>

English / [简体中文](README_zh.md)

<p align="center"><b>go-callflow-vis</b> is a tool for analyzing and visualizing complex software architecture hierarchies</p>

---

</div>

## Introduction

Traditional static analysis tools(like [go-callvis](https://github.com/ondrajz/go-callvis)) produce a complete call graph for a project. However, when dealing with complex projects, the result is often a tangled mess, difficult to discern, and loses practical value. In contrast, go-callflow-vis offers a more refined and controllable method for analyzing the architectural hierarchies of complex software.

go-callflow-vis allows users to define a series of ordered call hierarchies (for instance, in a typical Web project, these layers might be API->Operator->Manager->DAO) and enables specifying key functions or function categories for each layer through a configuration file. go-callflow-vis can analyze and visualize the reachability and call flow between these hierarchical functions. For two reachable functions across adjacent layers, go-callflow-vis only provides one example path to prevent the call graph from becoming overly complicated, ensuring that the functions' reachability is accurately represented.

## Features

- **Hierarchical Call Flow Output**: Focuses on the reachability and call flow between functions of adjacent layers, avoiding overly complex results.

- **Flexible Configuration**: Allows users to define key functions or function categories for each layer, enabling more precise project structure analysis.

- **Visualization and Interaction**: Offers excellent, interactive visual results, helping developers understand and optimize code structure more intuitively.

## Installation

```shell
go install github.com/laindream/go-callflow-vis@latest
```

## Usage

Here, we use the analysis of [go-ethereum](https://github.com/ethereum/go-ethereum) as an example (see the [example](example) directory for details).

- **Writing Configuration File**

Suppose you want to quickly analyze the call relationship to the MPT (Merkle Patricia Trie) DB during the creation of the genesis block in go-ethereum, you can write the configuration file as follows ([example.toml](example.toml) introduces how to make detailed configurations):

```toml
# file:init_genesis_analysis.toml

# package_prefix is for trimming the function name in graph for human readability
package_prefix = "github.com/ethereum/go-ethereum/"


# layer is a set of matched functions used to generate flow graph. layers must be defined in order.
[[layer]]
name = "CMD Layer"
[[layer.entities]]
# match rule for the function name
# there are match type: "contain", "prefix", "suffix", "equal", "regexp", default to use "equal" if not set type
# can set exclude = true to exclude the matched functions
name = { rules = [{ type = "suffix", content = "initGenesis" }] }


[[layer]]
name = "DB Layer"
[[layer.entities]]
name = { rules = [{ type = "contain", content = "triedb.Database" }] }
```

- **Starting the Analysis**

Next, assuming you have downloaded the source code of go-ethereum and installed go-callflow-vis; then, entering the cmd/geth directory, you can start the analysis with the following command (see the quick script in [go_eth_example.sh](example/go_eth_example.sh)):

```shell
# run go-callflow-vis directly to see detailed command usage
go-callflow-vis -config init_genesis_analysis.toml -web .
```

- **Viewing the Analysis Results**

If everything goes well, you will be able to see your browser pop up and display the visualized and interactive analysis results.

In addition, the program will output the analysis call graph([dot file](example/graph_out)) and the call chain list([csv file](example/path_out)), default location: `./graph_out` and `./path_out` .

You can also obtain visualized svg files from the call graph's dot files (requires installing [graphviz](https://graphviz.org/)).

Run the following command in the graph_out directory:

```shell
dot -Tsvg -o complete_callgraph.svg  complete_callgraph.dot
dot -Tsvg -o simple_callgraph.svg  simple_callgraph.dot
```

You will be able to see two versions of the call graph, the complete version and the simplified version.

Complete version:

![complete_callgraph](example/graph_out/complete_callgraph.svg)

Simplified version:

![simple_callgraph](example/graph_out/simple_callgraph.svg)
