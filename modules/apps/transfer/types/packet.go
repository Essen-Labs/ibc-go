package types

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/cosmos/gogoproto/proto"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec/unknownproto"

	ibcerrors "github.com/cosmos/ibc-go/v9/modules/core/errors"
	ibcexported "github.com/cosmos/ibc-go/v9/modules/core/exported"
)

var (
	_ ibcexported.PacketData         = (*FungibleTokenPacketData)(nil)
	_ ibcexported.PacketDataProvider = (*FungibleTokenPacketData)(nil)
	_ ibcexported.PacketData         = (*FungibleTokenPacketDataV2)(nil)
	_ ibcexported.PacketDataProvider = (*FungibleTokenPacketDataV2)(nil)
)

// NewFungibleTokenPacketData constructs a new FungibleTokenPacketData instance
func NewFungibleTokenPacketData(
	denom string, amount string,
	sender, receiver string,
	memo string,
) FungibleTokenPacketData {
	return FungibleTokenPacketData{
		Denom:    denom,
		Amount:   amount,
		Sender:   sender,
		Receiver: receiver,
		Memo:     memo,
	}
}

// ValidateBasic is used for validating the token transfer.
// NOTE: The addresses formats are not validated as the sender and recipient can have different
// formats defined by their corresponding chains that are not known to IBC.
func (ftpd FungibleTokenPacketData) ValidateBasic() error {
	amount, ok := sdkmath.NewIntFromString(ftpd.Amount)
	if !ok {
		return errorsmod.Wrapf(ErrInvalidAmount, "unable to parse transfer amount (%s) into math.Int", ftpd.Amount)
	}
	if !amount.IsPositive() {
		return errorsmod.Wrapf(ErrInvalidAmount, "amount must be strictly positive: got %d", amount)
	}
	if strings.TrimSpace(ftpd.Sender) == "" {
		return errorsmod.Wrap(ibcerrors.ErrInvalidAddress, "sender address cannot be blank")
	}
	if strings.TrimSpace(ftpd.Receiver) == "" {
		return errorsmod.Wrap(ibcerrors.ErrInvalidAddress, "receiver address cannot be blank")
	}
	denom := ExtractDenomFromPath(ftpd.Denom)
	return denom.Validate()
}

// GetBytes is a helper for serialising the packet to bytes.
// The memo field of FungibleTokenPacketData is marked with the JSON omitempty tag
// ensuring that the memo field is not included in the marshalled bytes if one is not specified.
func (ftpd FungibleTokenPacketData) GetBytes() []byte {
	bz, err := json.Marshal(ftpd)
	if err != nil {
		panic(errors.New("cannot marshal FungibleTokenPacketData into bytes"))
	}

	return bz
}

// GetPacketSender returns the sender address embedded in the packet data.
//
// NOTE:
//   - The sender address is set by the module which requested the packet to be sent,
//     and this module may not have validated the sender address by a signature check.
//   - The sender address must only be used by modules on the sending chain.
//   - sourcePortID is not used in this implementation.
func (ftpd FungibleTokenPacketData) GetPacketSender(sourcePortID string) string {
	return ftpd.Sender
}

// GetCustomPacketData interprets the memo field of the packet data as a JSON object
// and returns the value associated with the given key.
// If the key is missing or the memo is not properly formatted, then nil is returned.
func (ftpd FungibleTokenPacketData) GetCustomPacketData(key string) interface{} {
	if len(ftpd.Memo) == 0 {
		return nil
	}

	jsonObject := make(map[string]interface{})
	err := json.Unmarshal([]byte(ftpd.Memo), &jsonObject)
	if err != nil {
		return nil
	}

	memoData, found := jsonObject[key]
	if !found {
		return nil
	}

	return memoData
}

// NewFungibleTokenPacketDataV2 constructs a new FungibleTokenPacketDataV2 instance
func NewFungibleTokenPacketDataV2(
	tokens []Token,
	sender, receiver string,
	memo string,
	forwarding ForwardingPacketData,
) FungibleTokenPacketDataV2 {
	return FungibleTokenPacketDataV2{
		Tokens:     tokens,
		Sender:     sender,
		Receiver:   receiver,
		Memo:       memo,
		Forwarding: forwarding,
	}
}

// ValidateBasic is used for validating the token transfer.
// NOTE: The addresses formats are not validated as the sender and recipient can have different
// formats defined by their corresponding chains that are not known to IBC.
func (ftpd FungibleTokenPacketDataV2) ValidateBasic() error {
	if strings.TrimSpace(ftpd.Sender) == "" {
		return errorsmod.Wrap(ibcerrors.ErrInvalidAddress, "sender address cannot be blank")
	}

	if strings.TrimSpace(ftpd.Receiver) == "" {
		return errorsmod.Wrap(ibcerrors.ErrInvalidAddress, "receiver address cannot be blank")
	}

	if len(ftpd.Tokens) == 0 {
		return errorsmod.Wrap(ErrInvalidAmount, "tokens cannot be empty")
	}

	for _, token := range ftpd.Tokens {
		if err := token.Validate(); err != nil {
			return err
		}
	}

	if len(ftpd.Memo) > MaximumMemoLength {
		return errorsmod.Wrapf(ErrInvalidMemo, "memo must not exceed %d bytes", MaximumMemoLength)
	}

	if err := ftpd.Forwarding.Validate(); err != nil {
		return err
	}

	// We cannot have non-empty memo and non-empty forwarding path hops at the same time.
	if ftpd.HasForwarding() && ftpd.Memo != "" {
		return errorsmod.Wrapf(ErrInvalidMemo, "memo must be empty if forwarding path hops is not empty: %s, %s", ftpd.Memo, ftpd.Forwarding.Hops)
	}

	return nil
}

// GetBytes is a helper for serialising a FungibleTokenPacketDataV2. It uses protobuf to serialise
// the packet data and panics on failure.
func (ftpd FungibleTokenPacketDataV2) GetBytes() []byte {
	bz, err := proto.Marshal(&ftpd)
	if err != nil {
		panic(errors.New("cannot marshal FungibleTokenPacketDataV2 into bytes"))
	}

	return bz
}

// GetCustomPacketData interprets the memo field of the packet data as a JSON object
// and returns the value associated with the given key.
// If the key is missing or the memo is not properly formatted, then nil is returned.
func (ftpd FungibleTokenPacketDataV2) GetCustomPacketData(key string) interface{} {
	if len(ftpd.Memo) == 0 {
		return nil
	}

	jsonObject := make(map[string]interface{})
	err := json.Unmarshal([]byte(ftpd.Memo), &jsonObject)
	if err != nil {
		return nil
	}

	memoData, found := jsonObject[key]
	if !found {
		return nil
	}

	return memoData
}

// GetPacketSender returns the sender address embedded in the packet data.
//
// NOTE:
//   - The sender address is set by the module which requested the packet to be sent,
//     and this module may not have validated the sender address by a signature check.
//   - The sender address must only be used by modules on the sending chain.
//   - sourcePortID is not used in this implementation.
func (ftpd FungibleTokenPacketDataV2) GetPacketSender(sourcePortID string) string {
	return ftpd.Sender
}

// HasForwarding determines if the packet should be forwarded to the next hop.
func (ftpd FungibleTokenPacketDataV2) HasForwarding() bool {
	return len(ftpd.Forwarding.Hops) > 0
}

// UnmarshalPacketData attempts to unmarshal the provided packet data bytes into a FungibleTokenPacketDataV2.
// The version of ics20 should be provided and should be either ics20-1 or ics20-2.
func UnmarshalPacketData(bz []byte, ics20Version string) (FungibleTokenPacketDataV2, error) {
	// TODO: in transfer ibc module V2, we need to respect he encoding value passed via the payload, some hard coded assumptions about
	// encoding exist here based on the ics20 version passed in.
	switch ics20Version {
	case V1:
		var datav1 FungibleTokenPacketData
		if err := json.Unmarshal(bz, &datav1); err != nil {
			return FungibleTokenPacketDataV2{}, errorsmod.Wrapf(ibcerrors.ErrInvalidType, "cannot unmarshal ICS20-V1 transfer packet data: %s", err.Error())
		}

		return PacketDataV1ToV2(datav1)
	case V2:
		var datav2 FungibleTokenPacketDataV2
		if err := unknownproto.RejectUnknownFieldsStrict(bz, &datav2, unknownproto.DefaultAnyResolver{}); err != nil {
			return FungibleTokenPacketDataV2{}, errorsmod.Wrapf(ibcerrors.ErrInvalidType, "cannot unmarshal ICS20-V2 transfer packet data: %s", err.Error())
		}

		if err := proto.Unmarshal(bz, &datav2); err != nil {
			return FungibleTokenPacketDataV2{}, errorsmod.Wrapf(ibcerrors.ErrInvalidType, "cannot unmarshal ICS20-V2 transfer packet data: %s", err.Error())
		}

		if err := datav2.ValidateBasic(); err != nil {
			return FungibleTokenPacketDataV2{}, err
		}

		return datav2, nil
	default:
		return FungibleTokenPacketDataV2{}, errorsmod.Wrap(ErrInvalidVersion, ics20Version)
	}
}

// PacketDataV1ToV2 converts a v1 packet data to a v2 packet data. The packet data is validated
// before conversion.
func PacketDataV1ToV2(packetData FungibleTokenPacketData) (FungibleTokenPacketDataV2, error) {
	if err := packetData.ValidateBasic(); err != nil {
		return FungibleTokenPacketDataV2{}, errorsmod.Wrapf(err, "invalid packet data")
	}

	denom := ExtractDenomFromPath(packetData.Denom)
	return FungibleTokenPacketDataV2{
		Tokens: []Token{
			{
				Denom:  denom,
				Amount: packetData.Amount,
			},
		},
		Sender:     packetData.Sender,
		Receiver:   packetData.Receiver,
		Memo:       packetData.Memo,
		Forwarding: ForwardingPacketData{},
	}, nil
}
