package ibctesting

import (
	"github.com/stretchr/testify/require"

	channeltypesv2 "github.com/cosmos/ibc-go/v9/modules/core/04-channel/v2/types"
	hostv2 "github.com/cosmos/ibc-go/v9/modules/core/24-host/v2"
)

// MsgSendPacket sends a packet on the associated endpoint. The constructed packet is returned.
func (endpoint *Endpoint) MsgSendPacket(timeoutTimestamp uint64, packetData channeltypesv2.PacketData) (channeltypesv2.Packet, error) {
	msgSendPacket := channeltypesv2.NewMsgSendPacket(endpoint.ChannelID, timeoutTimestamp, endpoint.Chain.SenderAccount.GetAddress().String(), packetData)

	_, err := endpoint.Chain.SendMsgs(msgSendPacket)
	if err != nil {
		return channeltypesv2.Packet{}, err
	}

	if err := endpoint.Counterparty.UpdateClient(); err != nil {
		return channeltypesv2.Packet{}, err
	}

	// TODO: parse the packet from events instead of manually constructing it. https://github.com/cosmos/ibc-go/issues/7459
	nextSequenceSend, ok := endpoint.Chain.GetSimApp().IBCKeeper.ChannelKeeperV2.GetNextSequenceSend(endpoint.Chain.GetContext(), endpoint.ChannelID)
	require.True(endpoint.Chain.TB, ok)
	packet := channeltypesv2.NewPacket(nextSequenceSend-1, endpoint.ChannelID, endpoint.Counterparty.ChannelID, timeoutTimestamp, packetData)

	return packet, nil
}

// MsgRecvPacket sends a MsgRecvPacket on the associated endpoint with the provided packet.
func (endpoint *Endpoint) MsgRecvPacket(packet channeltypesv2.Packet) error {
	// get proof of packet commitment from chainA
	packetKey := hostv2.PacketCommitmentKey(packet.SourceChannel, packet.Sequence)
	proof, proofHeight := endpoint.Counterparty.QueryProof(packetKey)

	msg := channeltypesv2.NewMsgRecvPacket(packet, proof, proofHeight, endpoint.Chain.SenderAccount.GetAddress().String())

	if err := endpoint.Chain.sendMsgs(msg); err != nil {
		return err
	}

	return endpoint.Counterparty.UpdateClient()
}

// MsgAcknowledgePacket sends a MsgAcknowledgement on the associated endpoint with the provided packet and ack.
func (endpoint *Endpoint) MsgAcknowledgePacket(packet channeltypesv2.Packet, ack channeltypesv2.Acknowledgement) error {
	packetKey := hostv2.PacketAcknowledgementKey(packet.DestinationChannel, packet.Sequence)
	proof, proofHeight := endpoint.Counterparty.QueryProof(packetKey)

	msg := channeltypesv2.NewMsgAcknowledgement(packet, ack, proof, proofHeight, endpoint.Chain.SenderAccount.GetAddress().String())

	if err := endpoint.Chain.sendMsgs(msg); err != nil {
		return err
	}

	return endpoint.Counterparty.UpdateClient()
}