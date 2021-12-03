package utxo

const (
	UTXOStoreKeyPrefixLedgerMilestoneIndex byte = 0

	UTXOStoreKeyPrefixOutput      byte = 1 //TODO: iterate over all values and map to extended outputs
	UTXOStoreKeyPrefixOutputSpent byte = 8

	UTXOStoreKeyPrefixMilestoneDiffs byte = 4

	// Chrysalis Migration
	UTXOStoreKeyPrefixTreasuryOutput byte = 6
	UTXOStoreKeyPrefixReceipts       byte = 7

	// ExtendedOutput and Alias controllers
	UTXOStoreKeyPrefixOutputOnAddressUnspent byte = 9
	UTXOStoreKeyPrefixOutputOnAddressSpent   byte = 10

	// AliasOutputs
	UTXOStoreKeyPrefixAliasUnspent byte = 11
	UTXOStoreKeyPrefixAliasSpent   byte = 12

	// NFTOutputs
	UTXOStoreKeyPrefixNFTUnspent byte = 13
	UTXOStoreKeyPrefixNFTSpent   byte = 14

	// FoundryOutputs
	UTXOStoreKeyPrefixFoundryUnspent byte = 15
	UTXOStoreKeyPrefixFoundrySpent   byte = 16

	// Feature Block lookups
	UTXOStoreKeyPrefixIssuerLookup         byte = 17
	UTXOStoreKeyPrefixSenderLookup         byte = 18
	UTXOStoreKeyPrefixSenderAndIndexLookup byte = 19
)

// Deprecated keys, just used for migration purposes
const (
	UTXOStoreKeyPrefixUnspent  byte = 2 //TODO: migrate to UTXOStoreKeyPrefixOutputOnAddressUnspent and drop
	UTXOStoreKeyPrefixSpent    byte = 3 //TODO: migrate to UTXOStoreKeyPrefixOutputOnAddressSpent and UTXOStoreKeyPrefixSpent, then drop
	UTXOStoreKeyPrefixBalances byte = 5 //TODO: deprecate and drop
)

// https://hackmd.io/SDbu_YCZTuOH4QhABf6ViQ?view

/*
	Required LUTs:

-   outputID (value: output)

hasSpendingConstraints: dust-deposit-return, expiration-lock, time-lock

- (spent/unspent) address + hasSpendingConstraints + outputType + outputID
	* target-address (ed25519, alias, nft)
	* state controller (ed25519, alias)
	* governance controller (ed25519, alias)
- 	(spent/unspent) aliasID + outputID
-   (spent/unspent) nftID + outputID
- 	(spent/unspent) foundryID + outputID

-	issuer + type + outputID
	* issuer (ed25519, alias, nft)
-	sender + type + outputID
	* sender (ed25519, alias, nft)
-	sender + index + type + outputID
	* sender (ed25519, alias, nft)
*/

/*

   UTXO Database

   MilestoneIndex:
   ===============
   Key:
       UTXOStoreKeyPrefixLedgerMilestoneIndex
                    1 byte

   Value:
       milestone.Index
          4 bytes


   Output:
   =======
   Key:
       UTXOStoreKeyPrefixOutput + iotago.UTXOInputID
                   1 byte       + 32 bytes + 2 bytes

   Value:
       MessageID + iotago.Output.Serialized()
        32 bytes +    4 bytes type + X bytes


   Unspent Output:
   ===============
   Key:
       UTXOStoreKeyPrefixUnspent +     iotago.Address.Serialized()       + iotago.OutputType + iotago.OutputID
                 1 byte          +       1 byte type + 20-32 bytes       +       1 bytes     + 32 bytes + 2 bytes

   Value:
       Empty


   Spent Output:
   ================
   Key:
       UTXOStoreKeyPrefixSpent +       iotago.Address.Serialized()     + iotago.OutputType + iotago.OutputID
                 1 byte        +       1 byte type + 20-32 bytes       +       1 byte      + 32 bytes + 2 bytes

   Value:
       TargetTransactionID (iotago.TransactionID) + ConfirmationIndex (milestone.Index)
                  32 bytes                        +            4 bytes


   Treasury Output:
   =======
   Key:
       UTXOStoreKeyPrefixTreasuryOutput + spent  + milestone hash
                   1 byte               + 1 byte +    32 bytes

   Receipts:
   =======
   Key:
       UTXOStoreKeyPrefixReceipts + migrated_at_index  + milestone_index
                   1 byte         +      32 byte       +    32 bytes

   Value:
       Amount
       8 bytes

   Milestone diffs:
   ================
   Key:
       UTXOStoreKeyPrefixMilestoneDiffs + milestone.Index
                 1 byte                 +     4 bytes

   Value:
       OutputCount  +  OutputCount *  iotago.OutputID   + SpentCount + SpentCount *    iotago.OutputID    + has treasury +  TreasuryOutputMilestoneID  + SpentTreasuryOutputMilestoneID
         4 bytes    +  OutputCount * (32 byte + 2 byte) +   4 bytes  + SpentCount *  (32 bytes + 2 bytes) +    1 byte    +          32 bytes           +          32 bytes

*/
