# dna-bench
test dna tps, only support oep-4 token transfer bench at current.

## requet
1. deploy a oep-4 contract
2. account at wallet.dat has enough gas and oep-4 token that you deployed

## config
```
{
  "Wallet": "wallet.dat", // wallet path
  "Password": "123456", // account password
  "ContractCodePath": "myt.avm", // contract vm code
  "Contract": "4ef9e947c975eacb60c75b542c8d2fea36e09c65", // contract address
  "To": "AdTgdGPjahJjubZU19AwBu9F3oE4hncx4u", // to account
  "Amount": 1, // amount per transfer
  "GasPrice": 500,
  "GasLimit": 20000,
  "Rpc": [
    "http://polaris1.dna.io:20336",
    "http://polaris2.dna.io:20336",
    "http://polaris3.dna.io:20336",
    "http://polaris4.dna.io:20336"
  ], // dna network rpc address
  "ConsensusPeerPath": [ // consensus node wallet
      [
        "peer1.dat",
        "123456"
      ],
      [
        "peer2.dat",
        "123456"
      ],
      [
        "peer3.dat",
        "123456"
      ],
      [
        "peer4.dat",
        "123456"
      ],
      [
        "peer5.dat",
        "123456"
      ],
      [
        "peer6.dat",
        "123456"
      ],
      [
        "peer7.dat",
        "123456"
      ]
    ],
  "TxNum": 100, // tx num per factor
  "TxFactory": 1, // tx factor
  "RoutineNum": 2, // use go routine to handle bench
  "TPS": 10, // bench tps
  "SaveTx": true, // should save bench tx to file
  "SendTx": true // should send tx
}
```

If SendTx is true, tx will be sent to Rpc address, if SaveTx is true, tx will be save to file. If you want to only save tx rather than sent it, should set SaveTx true and SendTx false.

The program will use RoutineNum go routines to handle TxNum * TxFactor transaction.

## Usage

build:
``` go build -o main main.go ```

run:

case "init":
 
if you deployed a dna private network, you would:
1. withdraw gas from multi-sign address construct by whole consensus node;
2. set gas price to 500 to charge transaction fee to governance contract;
3. deploy your oep4 token;

``` ./main init config.json```

case "test-oep4"

```./main test-oep4 config.json```

case "test-gas"

```./main test-gas config.json```

case "balanceOf"

only query oep4 token balance now

```./main balanceOf config.json APPUBiR8LUVa6ZkGfQ7RG3VvuYYBqA9D4h```