echo $NODE_ID
wallet_miner=$(./blockchain_go createwallet | awk '{print $4}')

./blockchain_go startnode -miner $wallet_miner -interactive true
# ./blockchain_go startnode -miner $wallet_miner
