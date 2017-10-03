#### Building ####

`go get -u --tags nopkcs11 github.com/hyperledger/fabric/core/chaincode/shim`

`go build --tags nopkcs11`

#### Testing ####

Sample segments are stored in json files.

`go test --tags nopkcs11`