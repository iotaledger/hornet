package utxo

const (
	UTXOStoreKeyPrefixLedgerMilestoneIndex byte = 0
	UTXOStoreKeyPrefixOutput               byte = 1
	UTXOStoreKeyPrefixUnspent              byte = 2
	UTXOStoreKeyPrefixSpent                byte = 3
	UTXOStoreKeyPrefixMilestoneDiffs       byte = 4
	UTXOStoreKeyPrefixBalances             byte = 5 //TODO: deprecate and drop
	UTXOStoreKeyPrefixTreasuryOutput       byte = 6
	UTXOStoreKeyPrefixReceipts             byte = 7
)

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
