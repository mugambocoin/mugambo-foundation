#!/bin/bash

NODE_PATH="/root/Fork/nodes"
LOG_PATH="/root/Fork/nodes/logs"
GENESIS_PATH="/root/genesis"

"$NODE_PATH"/mugambo --datadir="$NODE_PATH"/rpc --genesis="$GENESIS_PATH"/testnet.g --port=9009 --nat extip:0.0.0.0 --http --http.addr="0.0.0.0" --http.port=80 --http.corsdomain="*" --http.api="eth,debug,net,admin,web3,personal,txpool,mgb,dag" --ws --ws.addr="0.0.0.0" --ws.port=81 --ws.origins="*" --ws.api="eth,debug,net,admin,web3,personal,txpool,mgb,dag" --nousb --verbosity=3 --tracing >> "$LOG_PATH"/rpc.log --ipcpath=\\.\pipe\rpc.ipc