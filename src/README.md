Step to reproduce broadcasting bug...

Open 3 terminals

1st Terminal:
export NODE_ID=3000
./startup.sh

2nd Terminal:
export NODE_ID=3001

3rd Terminal:
export NODE_ID=3002
./newminer.sh

2nd Terminal, immediately run
./blockchain_go startnode -interactive

You should see 'Add block...' message 2 times.

then wait 2-3 minutes.

2nd Terminal, CTRL-C to kill the process. wait 5 seconds until it found dead. run...
rm -rf blockchain_3001.db; cp blockchain_genesis.db blockchain_3001.db; ./blockchain_go startnode -interactive

This time, you should see it 'Send version' log in 2nd Terminal but the others won't print 'OnBroadcast Triggered'
