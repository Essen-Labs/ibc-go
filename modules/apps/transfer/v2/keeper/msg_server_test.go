package keeper_test

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v9/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v9/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v9/modules/core/04-channel/v2/types"
	ibctesting "github.com/cosmos/ibc-go/v9/testing"
)

// TestMsgTransfer tests Transfer rpc handler
func (suite *KeeperTestSuite) TestMsgSendPacketTransfer() {
	var payload channeltypesv2.Payload
	var path *ibctesting.Path

	testCases := []struct {
		name     string
		malleate func()
		expError error
	}{
		{
			"success",
			func() {},
			nil,
		},
		{
			"failure: send transfers disabled",
			func() {
				suite.chainA.GetSimApp().TransferKeeperV2.SetParams(suite.chainA.GetContext(),
					transfertypes.Params{
						SendEnabled: false,
					},
				)
			},
			transfertypes.ErrSendDisabled,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest()

			path = ibctesting.NewPath(suite.chainA, suite.chainB)
			path.SetupV2()

			tokens := []transfertypes.Token{
				{
					Denom: transfertypes.Denom{
						Base:  sdk.DefaultBondDenom,
						Trace: nil,
					},
					Amount: ibctesting.DefaultCoinAmount.String(),
				},
			}

			ftpd := transfertypes.NewFungibleTokenPacketDataV2(tokens, suite.chainA.SenderAccount.GetAddress().String(), suite.chainB.SenderAccount.GetAddress().String(), "", transfertypes.ForwardingPacketData{})
			bz := suite.chainA.Codec.MustMarshal(&ftpd)

			timestamp := suite.chainA.GetTimeoutTimestamp()
			// TODO: note, encoding field currently not respected in the implementation. encoding is determined by the version.
			// ics20-v1 == json
			// ics20-v2 == proto
			payload = channeltypesv2.NewPayload(transfertypes.ModuleName, transfertypes.ModuleName, transfertypes.V2, "json", bz)

			tc.malleate()

			packet, err := path.EndpointA.MsgSendPacket(timestamp, payload)

			expPass := tc.expError == nil
			if expPass {

				// ensure every token sent is escrowed.
				for _, t := range tokens {
					escrowedAmount := suite.chainA.GetSimApp().TransferKeeperV2.GetTotalEscrowForDenom(suite.chainA.GetContext(), t.Denom.IBCDenom())
					expected, err := t.ToCoin()
					suite.Require().NoError(err)
					suite.Require().Equal(expected, escrowedAmount, "escrowed amount is not equal to expected amount")
				}
				suite.Require().NoError(err)
				suite.Require().NotEmpty(packet)
			} else {
				ibctesting.RequireErrorIsOrContains(suite.T(), err, tc.expError, "expected error %q but got %q", tc.expError, err)
				suite.Require().Empty(packet)
			}

		})
	}
}

func (suite *KeeperTestSuite) TestMsgRecvPacketTransfer() {
	var (
		path        *ibctesting.Path
		packet      channeltypesv2.Packet
		expectedAck channeltypesv2.Acknowledgement
	)

	testCases := []struct {
		name     string
		malleate func()
		expError error
	}{
		{
			"success",
			func() {},
			nil,
		},
		{
			"failure: invalid destination channel on received packet",
			func() {
				packet.DestinationChannel = ibctesting.InvalidID
			},
			channeltypesv2.ErrChannelNotFound,
		},
		{
			"failure: counter party channel does not match source channel",
			func() {
				packet.SourceChannel = ibctesting.InvalidID
			},
			channeltypes.ErrInvalidChannelIdentifier,
		},
		{
			"failure: receive is disabled",
			func() {

				expectedAck.AcknowledgementResults[0].RecvPacketResult = channeltypesv2.RecvPacketResult{
					Status:          channeltypesv2.PacketStatus_Failure,
					Acknowledgement: channeltypes.NewErrorAcknowledgement(transfertypes.ErrReceiveDisabled).Acknowledgement(),
				}

				suite.chainB.GetSimApp().TransferKeeperV2.SetParams(suite.chainB.GetContext(),
					transfertypes.Params{
						ReceiveEnabled: false,
					})
			},
			nil,
		},
		// TODO: async tests
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest()

			path = ibctesting.NewPath(suite.chainA, suite.chainB)
			path.SetupV2()

			tokens := []transfertypes.Token{
				{
					Denom: transfertypes.Denom{
						Base:  sdk.DefaultBondDenom,
						Trace: nil,
					},
					Amount: ibctesting.DefaultCoinAmount.String(),
				},
			}

			ftpd := transfertypes.NewFungibleTokenPacketDataV2(tokens, suite.chainA.SenderAccount.GetAddress().String(), suite.chainB.SenderAccount.GetAddress().String(), "", transfertypes.ForwardingPacketData{})
			bz := suite.chainA.Codec.MustMarshal(&ftpd)

			timestamp := suite.chainA.GetTimeoutTimestamp()
			payload := channeltypesv2.NewPayload(transfertypes.ModuleName, transfertypes.ModuleName, transfertypes.V2, "json", bz)
			var err error
			packet, err = path.EndpointA.MsgSendPacket(timestamp, payload)

			// by default, we assume a successful acknowledgement will be written.
			expectedAck = channeltypesv2.Acknowledgement{AcknowledgementResults: []channeltypesv2.AcknowledgementResult{
				{
					AppName: transfertypes.ModuleName,
					RecvPacketResult: channeltypesv2.RecvPacketResult{
						Status:          channeltypesv2.PacketStatus_Success,
						Acknowledgement: channeltypes.NewResultAcknowledgement([]byte{byte(1)}).Acknowledgement(),
					},
				},
			}}

			tc.malleate()

			err = path.EndpointB.MsgRecvPacket(packet)

			expPass := tc.expError == nil
			if expPass {
				suite.Require().NoError(err)

				actualAckHash := suite.chainB.GetSimApp().IBCKeeper.ChannelKeeperV2.GetPacketAcknowledgement(suite.chainB.GetContext(), packet.DestinationChannel, packet.Sequence)
				expectedHash := channeltypesv2.CommitAcknowledgement(expectedAck)

				suite.Require().Equal(expectedHash, actualAckHash)

				denom := transfertypes.Denom{
					Base: sdk.DefaultBondDenom,
					Trace: []transfertypes.Hop{
						transfertypes.NewHop(payload.DestinationPort, packet.DestinationChannel),
					},
				}

				actualBalance := path.EndpointB.Chain.GetSimApp().TransferKeeperV2.BankKeeper.GetBalance(suite.chainB.GetContext(), suite.chainB.SenderAccount.GetAddress(), denom.IBCDenom())

				var expectedBalance sdk.Coin
				// on a successful ack we expect the full amount to be transferred
				if expectedAck.AcknowledgementResults[0].RecvPacketResult.Status == channeltypesv2.PacketStatus_Success {
					expectedBalance = sdk.NewCoin(denom.IBCDenom(), ibctesting.DefaultCoinAmount)
				} else {
					// otherwise the tokens do not make it to the address.
					expectedBalance = sdk.NewCoin(denom.IBCDenom(), sdkmath.NewInt(0))
				}

				suite.Require().Equal(expectedBalance.Amount, actualBalance.Amount)

			} else {
				ibctesting.RequireErrorIsOrContains(suite.T(), err, tc.expError, "expected error %q but got %q", tc.expError, err)
			}

		})
	}
}
