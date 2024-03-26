git clone --filter=blob:none --no-checkout --single-branch --branch master https://github.com/ethereum/go-ethereum.git

cd go-ethereum || exit

git sparse-checkout init --no-cone

git sparse-checkout set '*.go' '*go.mod' '*go.sum' '*.c' '*.h'

git checkout

cd cmd/geth || exit

go install github.com/laindream/go-callflow-vis@latest

go-callflow-vis -config ../../../init_genesis_analysis.toml -web -debug -out-dir ../../.. .
