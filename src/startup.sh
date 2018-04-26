go build -o blockchain_go *.go
rm -rf blockchain_genesis.db blockchain_3000.db blockchain_3001.db blockchain_3002.db blockchain_3003.db
rm -rf wallet_3000.dat wallet_3001.dat wallet_3002.dat wallet_3003.dat

export NODE_ID=3000
wallet_3000_1=$(./blockchain_go createwallet | awk '{print $4}')
./blockchain_go createblockchain -address $wallet_3000_1
cp blockchain_3000.db blockchain_genesis.db
cp blockchain_3000.db blockchain_3001.db
cp blockchain_3000.db blockchain_3002.db
cp blockchain_3000.db blockchain_3003.db

export NODE_ID=3001
wallet_3001_1=$(./blockchain_go createwallet | awk '{print $4}')
wallet_3001_2=$(./blockchain_go createwallet | awk '{print $4}')
wallet_3001_3=$(./blockchain_go createwallet | awk '{print $4}')
wallet_3001_4=$(./blockchain_go createwallet | awk '{print $4}')

export NODE_ID=3000
amount=10
./blockchain_go send -from $wallet_3000_1 -to $wallet_3001_1 -amount $amount -mine
echo "Sent from $wallet_3000_1 to $wallet_3001_1 $amount coins"

./blockchain_go startnode -interactive true
# ./blockchain_go startnode 
