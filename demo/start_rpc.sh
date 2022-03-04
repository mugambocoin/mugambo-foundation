#!/bin/bash

NODE_PATH="/root/mugambo/nodes"
LOG_PATH="/root/mugambo/nodes/logs"

"$NODE_PATH"/mugambo --datadir="$NODE_PATH"/rpc --genesis="$NODE_PATH"/mainnet.g --port=9009 --nat extip:0.0.0.0 --http --http.addr="0.0.0.0" --http.port=80 --http.corsdomain="*" --http.api="eth,debug,net,admin,web3,personal,txpool,znx,dag" --ws --ws.addr="0.0.0.0" --ws.port=81 --ws.origins="*" --ws.api="eth,debug,net,admin,web3,personal,txpool,znx,dag" --nousb --verbosity=3 --tracing >> "$LOG_PATH"/rpc.log --ipcpath=\\.\pipe\rpc.ipc