package txbuilder

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/dotbitHQ/das-lib/common"
	"github.com/dotbitHQ/das-lib/core"
	"github.com/dotbitHQ/das-lib/witness"
	"github.com/nervosnetwork/ckb-sdk-go/address"
	"github.com/nervosnetwork/ckb-sdk-go/types"
	"github.com/shopspring/decimal"
)

func (d *DasTxBuilder) BuildMMJsonObj(evmChainId int64) (*common.MMJsonObj, error) {
	var res common.MMJsonObj
	if err := json.Unmarshal([]byte(common.MMJsonObjStr), &res); err != nil {
		return nil, fmt.Errorf("json.Unmarshal err: %s", err.Error())
	}

	res.Domain.ChainID = evmChainId
	if res.Domain.ChainID == 0 {
		res.Domain.ChainID = 1
		if d.dasCore.NetType() != common.DasNetTypeMainNet {
			res.Domain.ChainID = 17000
		}
	}

	inputsCapacity, err := d.getCapacityFromInputs()
	if err != nil {
		return nil, fmt.Errorf("getCapacityFromInputs err: %s", err.Error())
	}
	outputsCapacity := d.Transaction.OutputsCapacity()
	feeCapacity := inputsCapacity - outputsCapacity

	res.Message.InputsCapacity = fmt.Sprintf("%s CKB", common.Capacity2Str(inputsCapacity))
	res.Message.OutputsCapacity = fmt.Sprintf("%s CKB", common.Capacity2Str(outputsCapacity))
	res.Message.Fee = fmt.Sprintf("%s CKB", common.Capacity2Str(feeCapacity))

	inputs, err := d.getInputsMMJsonCellInfo()
	if err != nil {
		return nil, fmt.Errorf("getInputsMMJsonCellInfo err: %s", err.Error())
	}
	outputs, err := d.getOutputsMMJsonCellInfo()
	if err != nil {
		return nil, fmt.Errorf("getOutputsMMJsonCellInfo err: %s", err.Error())
	}
	res.Message.Inputs = inputs
	res.Message.Outputs = outputs

	// must be the last one to be executed
	action, dasMessage, err := d.getMMJsonActionAndMessage()
	if err != nil {
		return nil, fmt.Errorf("getMMJsonActionAndMessage err: %s", err.Error())
	}
	res.Message.Action = action
	res.Message.DasMessage = dasMessage

	return &res, nil
}

func (d *DasTxBuilder) getInputsMMJsonCellInfo() ([]common.MMJsonCellInfo, error) {
	var cellList []*types.CellInfo
	for _, v := range d.Transaction.Inputs {
		item, err := d.getInputCell(v.PreviousOutput)
		if err != nil {
			return nil, fmt.Errorf("getInputCell err: %s", err.Error())
		}
		cellList = append(cellList, item.Cell)
	}
	return d.getMMJsonCellInfo(cellList, common.DataTypeOld)
}

func (d *DasTxBuilder) getOutputsMMJsonCellInfo() ([]common.MMJsonCellInfo, error) {
	var cellList []*types.CellInfo
	for i, output := range d.Transaction.Outputs {
		cell := types.CellInfo{
			Output: output,
			Data: &types.CellData{
				Content: d.Transaction.OutputsData[i],
			},
		}
		cellList = append(cellList, &cell)
	}
	return d.getMMJsonCellInfo(cellList, common.DataTypeNew)
}

func (d *DasTxBuilder) getMMJsonCellInfo(cellList []*types.CellInfo, dataType common.DataType) ([]common.MMJsonCellInfo, error) {
	list := make([]common.MMJsonCellInfo, 0)
	for _, v := range cellList {
		var item common.MMJsonCellInfo
		item.Capacity = fmt.Sprintf("%s CKB", common.Capacity2Str(v.Output.Capacity))
		if v.Data != nil {
			item.Data = common.GetMaxHashLenData(v.Data.Content)
		}
		item.ExtraData = ""
		item.TypeStr = ""
		item.LockStr = ""
		if v.Output.Type == nil {
			continue
		}
		if lockContractName, ok := core.DasContractByTypeIdMap[v.Output.Lock.CodeHash.Hex()]; ok {
			item.LockStr = common.GetMaxHashLenScript(v.Output.Lock, lockContractName)
		} else {
			item.LockStr = common.GetMaxHashLenScriptForNormalCell(v.Output.Lock)
		}
		if typeContractName, ok := core.DasContractByTypeIdMap[v.Output.Type.CodeHash.Hex()]; ok {
			if typeContractName == common.DasContractNameBalanceCellType {
				continue
			}
			item.TypeStr = common.GetMaxHashLenScript(v.Output.Type, typeContractName)
			switch typeContractName {
			case common.DasContractNameDidCellType:
				item.TypeStr = common.GetMaxHashLenScriptForNormalCell(v.Output.Type)
			case common.DasContractNameAccountSaleCellType:
				builder, err := witness.AccountSaleCellDataBuilderFromTx(d.Transaction, dataType)
				if err != nil {
					return nil, fmt.Errorf("AccountSaleCellDataBuilderFromTx err: %s", err.Error())
				}
				d.salePrice = builder.Price
			case common.DasContractNameAccountCellType:
				builder, err := witness.AccountCellDataBuilderFromTx(d.Transaction, dataType)
				if err != nil {
					return nil, fmt.Errorf("AccountCellDataBuilderFromTx err: %s", err.Error())
				}
				d.account = builder.Account
				expiredAt, err := common.GetAccountCellExpiredAtFromOutputData(v.Data.Content)
				if err != nil {
					return nil, fmt.Errorf("GetAccountCellExpiredAtFromOutputData err: %s", err.Error())
				}
				item.Data = fmt.Sprintf("{ account: %s, expired_at: %d }", d.account, expiredAt)
				item.ExtraData = fmt.Sprintf("{ status: %d, records_hash: %s }", builder.Status, common.Bytes2Hex(builder.RecordsHashBys))
			case common.DasContractNameReverseRecordCellType:
				d.account = string(v.Data.Content)
			case common.DASContractNameOfferCellType:
				d.offers++
			}
		}
		list = append(list, item)
	}
	return list, nil
}

func (d *DasTxBuilder) getMMJsonActionAndMessage() (*common.MMJsonAction, string, error) {
	var action common.MMJsonAction
	actionDataBuilder, err := witness.ActionDataBuilderFromTx(d.Transaction)
	if err != nil {
		return nil, "", fmt.Errorf("ActionDataBuilderFromTx err: %s", err.Error())
	}
	action.Action = actionDataBuilder.Action
	action.Params = actionDataBuilder.ParamsStr

	dasMessage := ""
	daf := core.DasAddressFormat{DasNetType: d.dasCore.NetType()}
	switch action.Action {
	case common.DasActionTransferDP:
		dasMessage, err = d.getTransferDPMsg()
		if err != nil {
			return nil, "", fmt.Errorf("getTransferDPMsg err: %s", err.Error())
		}
		//log.Info("getTransferDPMsg:", dasMessage)
	case common.DasActionBurnDP:
		dasMessage, err = d.getBurnDPMsg()
		if err != nil {
			return nil, "", fmt.Errorf("getBurnDPMsg err: %s", err.Error())
		}
		//log.Info("getBurnDPMsg:", dasMessage)
	case common.DasActionBidExpiredAccountAuction:
		dasMessage, err = d.getBidExpiredAccountAuctionMsg()
		if err != nil {
			return nil, "", fmt.Errorf("getBidExpiredAccountAuctionMsg err: %s", err.Error())
		}
	case common.DasActionEditManager:
		dasMessage = fmt.Sprintf("EDIT MANAGER OF ACCOUNT %s", d.account)
	case common.DasActionEditRecords:
		dasMessage = fmt.Sprintf("EDIT RECORDS OF ACCOUNT %s", d.account)
	case common.DasActionTransferAccount:
		builder, err := witness.AccountCellDataBuilderFromTx(d.Transaction, common.DataTypeNew)
		if err != nil {
			return nil, "", fmt.Errorf("AccountCellDataBuilderFromTx err: %s", err.Error())
		}
		if ownerNormal, _, err := daf.ArgsToNormal(d.Transaction.Outputs[builder.Index].Lock.Args); err != nil {
			return nil, "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
		} else {
			dasMessage = fmt.Sprintf("TRANSFER THE ACCOUNT %s TO %s", d.account, ownerNormal.AddressNormal)
		}
	case common.DasActionTransfer, common.DasActionWithdrawFromWallet:
		dasMessage, err = d.getWithdrawDasMessage()
		if err != nil {
			return nil, "", fmt.Errorf("getWithdrawDasMessage err: %s", err.Error())
		}
	case common.DasActionStartAccountSale:
		dasMessage = fmt.Sprintf("SELL %s FOR %s CKB", d.account, common.Capacity2Str(d.salePrice))
	case common.DasActionEditAccountSale:
		dasMessage = fmt.Sprintf("EDIT SALE INFO, CURRENT PRICE IS %s CKB", common.Capacity2Str(d.salePrice))
	case common.DasActionCancelAccountSale:
		dasMessage = fmt.Sprintf("CANCEL SALE OF %s", d.account)
	case common.DasActionBuyAccount:
		dasMessage = fmt.Sprintf("BUY %s WITH %s CKB", d.account, common.Capacity2Str(d.salePrice))
	case common.DasActionDeclareReverseRecord:
		if ownerNormal, _, err := daf.ArgsToNormal(d.Transaction.Outputs[0].Lock.Args); err != nil {
			return nil, "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
		} else {
			dasMessage = fmt.Sprintf("DECLARE A REVERSE RECORD FROM %s TO %s", ownerNormal.AddressNormal, d.account)
		}
	case common.DasActionRedeclareReverseRecord:
		if ownerNormal, _, err := daf.ArgsToNormal(d.Transaction.Outputs[0].Lock.Args); err != nil {
			return nil, "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
		} else {
			dasMessage = fmt.Sprintf("REDECLARE A REVERSE RECORD FROM %s TO %s", ownerNormal.AddressNormal, d.account)
		}
	case common.DasActionRetractReverseRecord:
		if ownerNormal, _, err := daf.ArgsToNormal(d.Transaction.Outputs[0].Lock.Args); err != nil {
			return nil, "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
		} else {
			dasMessage = fmt.Sprintf("RETRACT REVERSE RECORDS ON %s", ownerNormal.AddressNormal)
		}
	case common.DasActionMakeOffer:
		builder, err := witness.OfferCellDataBuilderFromTx(d.Transaction, common.DataTypeNew)
		if err != nil {
			return nil, "", fmt.Errorf("OfferCellDataBuilderFromTx err: %s", err.Error())
		}
		dasMessage = fmt.Sprintf("MAKE AN OFFER ON %s WITH %s CKB", builder.Account, common.Capacity2Str(builder.Price))
	case common.DasActionEditOffer:
		builderOld, err := witness.OfferCellDataBuilderFromTx(d.Transaction, common.DataTypeOld)
		if err != nil {
			return nil, "", fmt.Errorf("OfferCellDataBuilderFromTx err: %s", err.Error())
		}
		builder, err := witness.OfferCellDataBuilderFromTx(d.Transaction, common.DataTypeNew)
		if err != nil {
			return nil, "", fmt.Errorf("OfferCellDataBuilderFromTx err: %s", err.Error())
		}
		dasMessage = fmt.Sprintf("CHANGE THE OFFER ON %s FROM %s CKB TO %s CKB", builder.Account, common.Capacity2Str(builderOld.Price), common.Capacity2Str(builder.Price))
	case common.DasActionCancelOffer:
		dasMessage = fmt.Sprintf("CANCEL %d OFFER(S)", d.offers)
	case common.DasActionAcceptOffer:
		builder, err := witness.OfferCellDataBuilderFromTx(d.Transaction, common.DataTypeOld)
		if err != nil {
			return nil, "", fmt.Errorf("OfferCellDataBuilderFromTx err: %s", err.Error())
		}
		dasMessage = fmt.Sprintf("ACCEPT THE OFFER ON %s WITH %s CKB", builder.Account, common.Capacity2Str(builder.Price))
	case common.DasActionLockAccountForCrossChain:
		dasMessage = fmt.Sprintf("LOCK %s FOR CROSS CHAIN", d.account)
	case common.DasActionCreateApproval, common.DasActionDelayApproval:
		builder, err := witness.AccountCellDataBuilderFromTx(d.Transaction, common.DataTypeNew)
		if err != nil {
			return nil, "", fmt.Errorf("AccountCellDataBuilderFromTx err: %s", err.Error())
		}

		switch action.Action {
		case common.DasActionCreateApproval:
			switch builder.AccountApproval.Action {
			case witness.AccountApprovalActionTransfer:
				ownerHex, _, err := d.dasCore.Daf().ScriptToHex(builder.AccountApproval.Params.Transfer.ToLock)
				if err != nil {
					return nil, "", err
				}
				dasAddressNormal, err := d.dasCore.Daf().HexToNormal(ownerHex)
				if err != nil {
					return nil, "", err
				}
				sealedUntil := builder.AccountApproval.Params.Transfer.SealedUntil
				dasMessage = fmt.Sprintf("APPROVE TRANSFER %s TO %s AFTER %d", d.account, dasAddressNormal.AddressNormal, sealedUntil)
			}
		case common.DasActionDelayApproval:
			switch builder.AccountApproval.Action {
			case witness.AccountApprovalActionTransfer:
				sealedUntil := builder.AccountApproval.Params.Transfer.SealedUntil
				dasMessage = fmt.Sprintf("DELAY THE TRANSFER APPROVAL OF %s TO %d", d.account, sealedUntil)
			}
		}
	case common.DasActionRevokeApproval, common.DasActionFulfillApproval:
		builder, err := witness.AccountCellDataBuilderFromTx(d.Transaction, common.DataTypeOld)
		if err != nil {
			return nil, "", fmt.Errorf("AccountCellDataBuilderFromTx err: %s", err.Error())
		}
		switch action.Action {
		case common.DasActionRevokeApproval:
			switch builder.AccountApproval.Action {
			case witness.AccountApprovalActionTransfer:
				dasMessage = fmt.Sprintf("REVOKE THE TRANSFER APPROVAL OF %s", d.account)
			}
		case common.DasActionFulfillApproval:
			switch builder.AccountApproval.Action {
			case witness.AccountApprovalActionTransfer:
				ownerHex, _, err := d.dasCore.Daf().ScriptToHex(builder.AccountApproval.Params.Transfer.ToLock)
				if err != nil {
					return nil, "", err
				}
				dasAddressNormal, err := d.dasCore.Daf().HexToNormal(ownerHex)
				if err != nil {
					return nil, "", err
				}
				dasMessage = fmt.Sprintf("FULFILL THE TRANSFER APPROVAL OF %s, TRANSFER TO %s", d.account, dasAddressNormal.AddressNormal)
			}
		}
	default:
		return nil, "", fmt.Errorf("not support action[%s]", action)
	}

	return &action, dasMessage, nil
}

func (d *DasTxBuilder) getWithdrawDasMessage() (string, error) {
	dasLock, err := core.GetDasContractInfo(common.DasContractNameDispatchCellType)
	if err != nil {
		return "", fmt.Errorf("GetDasContractInfo err: %s", err.Error())
	}
	mod := address.Testnet
	if d.dasCore.NetType() == common.DasNetTypeMainNet {
		mod = address.Mainnet
	}

	var mapInputs = make(map[string]uint64)
	var sortListInputs = make([]string, 0)
	for _, v := range d.Transaction.Inputs {
		receiverAddr := ""
		item, err := d.getInputCell(v.PreviousOutput)
		if err != nil {
			return "", fmt.Errorf("getInputCell err: %s", err.Error())
		}
		if dasLock.IsSameTypeId(item.Cell.Output.Lock.CodeHash) {
			ownerNormal, _, err := d.dasCore.Daf().ArgsToNormal(item.Cell.Output.Lock.Args)
			if err != nil {
				return "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
			}
			receiverAddr = ownerNormal.AddressNormal
		} else {
			receiverAddr, _ = common.ConvertScriptToAddress(mod, item.Cell.Output.Lock)
		}
		if c, ok := mapInputs[receiverAddr]; ok {
			mapInputs[receiverAddr] = c + item.Cell.Output.Capacity
		} else {
			mapInputs[receiverAddr] = item.Cell.Output.Capacity
			sortListInputs = append(sortListInputs, receiverAddr)
		}
	}
	dasMessageInputs := ""
	for _, v := range sortListInputs {
		capacity := mapInputs[v]
		dasMessageInputs = dasMessageInputs + fmt.Sprintf("%s(%s CKB), ", v, common.Capacity2Str(capacity))
	}

	//inputsCapacity, err := d.getCapacityFromInputs()
	//if err != nil {
	//	return "", fmt.Errorf("getCapacityFromInputs err: %s", err.Error())
	//}
	//item, err := d.getInputCell(d.Transaction.Inputs[0].PreviousOutput)
	//if err != nil {
	//	return "", fmt.Errorf("getInputCell err: %s", err.Error())
	//}
	//
	//daf := core.DasAddressFormat{DasNetType: d.dasCore.NetType()}
	//ownerNormal, _, err := daf.ArgsToNormal(item.Cell.Output.Lock.Args)
	//if err != nil {
	//	return "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
	//}
	//dasMessage := fmt.Sprintf("%s(%s CKB) TO ", ownerNormal.AddressNormal, common.Capacity2Str(inputsCapacity))

	// need merge outputs the capacity with the same lock script
	var mapOutputs = make(map[string]uint64)
	var sortList = make([]string, 0)

	for _, v := range d.Transaction.Outputs {
		receiverAddr := ""
		if v.Lock.CodeHash.Hex() == dasLock.ContractTypeId.Hex() {
			ownerNormal, _, err := d.dasCore.Daf().ArgsToNormal(v.Lock.Args)
			if err != nil {
				return "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
			}
			receiverAddr = ownerNormal.AddressNormal
		} else {
			receiverAddr, _ = common.ConvertScriptToAddress(mod, v.Lock)
		}
		if c, ok := mapOutputs[receiverAddr]; ok {
			mapOutputs[receiverAddr] = c + v.Capacity
		} else {
			mapOutputs[receiverAddr] = v.Capacity
			sortList = append(sortList, receiverAddr)
		}
	}
	dasMessageOutputs := ""
	for _, v := range sortList {
		capacity := mapOutputs[v]
		dasMessageOutputs = dasMessageOutputs + fmt.Sprintf("%s(%s CKB), ", v, common.Capacity2Str(capacity))
	}

	return fmt.Sprintf("TRANSFER FROM %s TO %s", dasMessageInputs[:len(dasMessageInputs)-2], dasMessageOutputs[:len(dasMessageOutputs)-2]), nil
}

func (d *DasTxBuilder) getTransferDPMsg() (string, error) {
	contractDP, err := core.GetDasContractInfo(common.DasContractNameDpCellType)
	if err != nil {
		return "", fmt.Errorf("GetDasContractInfo err: %s", err.Error())
	}
	daf := core.DasAddressFormat{DasNetType: d.dasCore.NetType()}
	// inputs
	var inputsAddr string
	var inputsAmount uint64
	for _, v := range d.Transaction.Inputs {
		item, err := d.getInputCell(v.PreviousOutput)
		if err != nil {
			return "", fmt.Errorf("getInputCell err: %s", err.Error())
		}
		if item.Cell.Output.Type == nil {
			continue
		}
		if !contractDP.IsSameTypeId(item.Cell.Output.Type.CodeHash) {
			continue
		}
		addr, _, err := daf.ArgsToNormal(item.Cell.Output.Lock.Args)
		if err != nil {
			return "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
		}
		inputsAddr = addr.AddressNormal
		dpData, err := witness.ConvertBysToDPData(item.Cell.Data.Content)
		if err != nil {
			return "", fmt.Errorf("ConvertBysToDPData err: %s", err.Error())
		}
		inputsAmount += dpData.Value
	}

	// outputs
	var outputsMap = make(map[string]uint64)
	var sortList = make([]string, 0)
	for i, v := range d.Transaction.Outputs {
		if v.Type == nil {
			continue
		}
		if !contractDP.IsSameTypeId(v.Type.CodeHash) {
			continue
		}
		addr, _, err := daf.ArgsToNormal(v.Lock.Args)
		if err != nil {
			return "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
		}
		dpData, err := witness.ConvertBysToDPData(d.Transaction.OutputsData[i])
		if err != nil {
			return "", fmt.Errorf("molecule.Bytes2GoU64 err: %s", err.Error())
		}
		if item, ok := outputsMap[addr.AddressNormal]; ok {
			outputsMap[addr.AddressNormal] = item + dpData.Value
		} else {
			outputsMap[addr.AddressNormal] = dpData.Value
			sortList = append(sortList, addr.AddressNormal)
		}
	}
	msg := ""
	for _, v := range sortList {
		amountStr := decimal.NewFromInt(int64(outputsMap[v])).DivRound(decimal.NewFromInt(1e6), 6)
		msg += fmt.Sprintf("%s(%s DP), ", v, amountStr.String())
	}
	inputsAmountStr := decimal.NewFromInt(int64(inputsAmount)).DivRound(decimal.NewFromInt(1e6), 6)
	return fmt.Sprintf("TRANSFER FROM %s(%s DP) TO %s", inputsAddr, inputsAmountStr.String(), msg[:len(msg)-2]), nil
}

func (d *DasTxBuilder) getBurnDPMsg() (string, error) {
	contractDP, err := core.GetDasContractInfo(common.DasContractNameDpCellType)
	if err != nil {
		return "", fmt.Errorf("GetDasContractInfo err: %s", err.Error())
	}
	daf := core.DasAddressFormat{DasNetType: d.dasCore.NetType()}
	// inputs
	var inputsAddr string
	var inputsAmount uint64
	for _, v := range d.Transaction.Inputs {
		item, err := d.getInputCell(v.PreviousOutput)
		if err != nil {
			return "", fmt.Errorf("getInputCell err: %s", err.Error())
		}
		if item.Cell.Output.Type == nil {
			continue
		}
		if !contractDP.IsSameTypeId(item.Cell.Output.Type.CodeHash) {
			continue
		}
		addr, _, err := daf.ArgsToNormal(item.Cell.Output.Lock.Args)
		if err != nil {
			return "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
		}
		inputsAddr = addr.AddressNormal

		dpData, err := witness.ConvertBysToDPData(item.Cell.Data.Content)
		if err != nil {
			return "", fmt.Errorf("Bytes2GoU64 err: %s", err.Error())
		}
		inputsAmount += dpData.Value
	}
	// outputs
	var outputsMap = make(map[string]uint64)
	for i, v := range d.Transaction.Outputs {
		if v.Type == nil {
			continue
		}
		if !contractDP.IsSameTypeId(v.Type.CodeHash) {
			continue
		}
		addr, _, err := daf.ArgsToNormal(v.Lock.Args)
		if err != nil {
			return "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
		}
		dpData, err := witness.ConvertBysToDPData(d.Transaction.OutputsData[i])
		if err != nil {
			return "", fmt.Errorf("molecule.Bytes2GoU64 err: %s", err.Error())
		}
		if item, ok := outputsMap[addr.AddressNormal]; ok {
			outputsMap[addr.AddressNormal] = item + dpData.Value
		} else {
			outputsMap[addr.AddressNormal] = dpData.Value
		}
	}
	//
	if amount, ok := outputsMap[inputsAddr]; ok {
		inputsAmount -= amount
	}
	inputsAmountStr := decimal.NewFromInt(int64(inputsAmount)).DivRound(decimal.NewFromInt(1e6), 6)
	return fmt.Sprintf("BURN %s DP FROM %s", inputsAmountStr.String(), inputsAddr), nil
}

func (d *DasTxBuilder) getBidExpiredAccountAuctionMsg() (string, error) {
	contractDP, err := core.GetDasContractInfo(common.DasContractNameDpCellType)
	if err != nil {
		return "", fmt.Errorf("GetDasContractInfo err: %s", err.Error())
	}
	// inputs
	var inputsPayload string
	var inputsAmount uint64
	for _, v := range d.Transaction.Inputs {
		item, err := d.getInputCell(v.PreviousOutput)
		if err != nil {
			return "", fmt.Errorf("getInputCell err: %s", err.Error())
		}
		if item.Cell.Output.Type == nil {
			continue
		}
		if !contractDP.IsSameTypeId(item.Cell.Output.Type.CodeHash) {
			continue
		}
		//addr, _, err := d.dasCore.Daf().ArgsToNormal(item.Cell.Output.Lock.Args)
		//if err != nil {
		//	return "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
		//}
		//inputsAddr = addr.AddressNormal
		addrHex, _, err := d.dasCore.Daf().ArgsToHex(item.Cell.Output.Lock.Args)
		if err != nil {
			return "", fmt.Errorf("ArgsToNormal err: %s", err.Error())
		}
		inputsPayload = hex.EncodeToString(addrHex.AddressPayload)

		dpData, err := witness.ConvertBysToDPData(item.Cell.Data.Content)
		if err != nil {
			return "", fmt.Errorf("Bytes2GoU64 err: %s", err.Error())
		}
		inputsAmount += dpData.Value
	}
	// outputs
	outputsDP, err := d.dasCore.GetOutputsDPInfo(d.Transaction)
	if err != nil {
		return "", fmt.Errorf("GetOutputsDPInfo err: %s", err.Error())
	}
	if item, ok := outputsDP[inputsPayload]; ok {
		inputsAmount -= item.AmountDP
	}
	costAmount := decimal.NewFromInt(int64(inputsAmount)).DivRound(decimal.NewFromInt(1e6), 6)
	return fmt.Sprintf("BID EXPIRED ACCOUNT %s WITH %s DP", d.account, costAmount), nil
}
