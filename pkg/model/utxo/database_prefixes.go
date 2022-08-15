package utxo

const (
	UTXOStoreKeyPrefixLedgerMilestoneIndex byte = 0

	// UTXOStoreKeyPrefixOutput defines the prefix for Output and Spent storage.
	UTXOStoreKeyPrefixOutput byte = 1

	// UTXOStoreKeyPrefixOutputSpent defines the prefix for Track spent/unspent Outputs.
	UTXOStoreKeyPrefixOutputSpent   byte = 2
	UTXOStoreKeyPrefixOutputUnspent byte = 3

	// UTXOStoreKeyPrefixMilestoneDiffs defines the prefix for Milestone diffs.
	UTXOStoreKeyPrefixMilestoneDiffs byte = 4

	// UTXOStoreKeyPrefixTreasuryOutput defines the prefix for the Treasury Output.
	UTXOStoreKeyPrefixTreasuryOutput byte = 5
	UTXOStoreKeyPrefixReceipts       byte = 6
)

/*
   UTXO Database

   MilestoneIndex:
   ===============
   Key:
       UTXOStoreKeyPrefixLedgerMilestoneIndex
                    1 byte

   Value:
       iotago.MilestoneIndex
          4 bytes

   Output:
   =======
   Key:
       UTXOStoreKeyPrefixOutput + iotago.OutputID
                1 byte          +     34 bytes

   Value:
       BlockID + MilestoneIndex + MilestoneTimestamp + iotago.Output.Serialized()
        32 bytes +    4 bytes     +      4 bytes       +   1 byte type + X bytes

   Spent Output:
   ================
   Key:
       UTXOStoreKeyPrefixSpent + iotago.OutputID
                 1 byte        +     34 bytes

   Value:
       TargetTransactionID (iotago.TransactionID) + ConfirmationIndex (iotago.MilestoneIndex) + ConfirmationTimestamp
                  32 bytes                        +            4 bytes                  +       4 bytes

   Unspent Output:
   ===============
   Key:
       UTXOStoreKeyPrefixUnspent + iotago.OutputID
                 1 byte          +     34 bytes

   Value:
       Empty


   Milestone diffs:
   ================
   Key:
       UTXOStoreKeyPrefixMilestoneDiffs + iotago.MilestoneIndex
                 1 byte                 +     4 bytes

   Value:
       OutputCount  +  OutputCount  *  iotago.OutputID   + SpentCount +  SpentCount *    iotago.OutputID    + has treasury +  TreasuryOutputMilestoneID  + SpentTreasuryOutputMilestoneID
         4 bytes    +  (OutputCount *    34 bytes)       +   4 bytes  + (SpentCount *       34 bytes)       +    1 byte    +          32 bytes           +          32 bytes

   Treasury Output:
   =======
   Key:
       UTXOStoreKeyPrefixTreasuryOutput + spent  + iotago.MilestoneID
                   1 byte               + 1 byte +    32 bytes

   Value:
       Amount
       8 bytes

   Receipts:
   =======
   Key:
       UTXOStoreKeyPrefixReceipts + migrated_at_index  + milestone_index
                   1 byte         +      4 byte        +    4 bytes

   Value:
       Receipt (iotago.ReceiptMilestoneOpt.Serialized())
                1 byte type + X bytes
*/
