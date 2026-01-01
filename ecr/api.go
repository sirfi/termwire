package ecr

import (
	"fmt"
	"time"

	pb "github.com/sirfi/termwire/protocol"
)

// API provides high-level operations for ECR
type API struct {
	client *Client
}

// NewAPI creates a new ECR API instance
func NewAPI(config *Config) *API {
	return &API{
		client: NewClient(config),
	}
}

// Connect connects to the POS terminal
func (api *API) Connect() error {
	return api.client.Connect()
}

// Disconnect disconnects from the POS terminal
func (api *API) Disconnect() error {
	return api.client.Disconnect()
}

// IsConnected returns whether the API is connected
func (api *API) IsConnected() bool {
	return api.client.IsConnected()
}

// GetVersion retrieves the POS terminal version
func (api *API) GetVersion() (*pb.GetVersionResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GetVersionRequest{
			GetVersionRequest: &pb.GetVersionRequest{},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	if resp, ok := response.Body.(*pb.Message_GetVersionResponse); ok {
		return resp.GetVersionResponse, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// GetTerminalInfo retrieves the POS terminal information
func (api *API) GetTerminalInfo() (*pb.GetTerminalInfoResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GetTerminalInfoRequest{
			GetTerminalInfoRequest: &pb.GetTerminalInfoRequest{},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	if resp, ok := response.Body.(*pb.Message_GetTerminalInfoResponse); ok {
		return resp.GetTerminalInfoResponse, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// GetBanks retrieves the list of available banks
func (api *API) GetBanks() (*pb.GetBanksResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GetBanksRequest{
			GetBanksRequest: &pb.GetBanksRequest{},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	if resp, ok := response.Body.(*pb.Message_GetBanksResponse); ok {
		return resp.GetBanksResponse, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// InsertCard simulates card insertion and returns card data
func (api *API) InsertCard(transactionID string, amountCents uint32, currency string) (*pb.CardInsertionResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_CardInsertionRequest{
			CardInsertionRequest: &pb.CardInsertionRequest{
				TransactionAmountCents: amountCents,
				Currency:               currency,
				TransactionId:          transactionID,
			},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	if resp, ok := response.Body.(*pb.Message_CardInsertionResponse); ok {
		if resp.CardInsertionResponse.Code != "00" {
			return resp.CardInsertionResponse, fmt.Errorf("card insertion failed: %s", resp.CardInsertionResponse.Message)
		}
		return resp.CardInsertionResponse, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// ProcessPayment processes a payment with selected bank
func (api *API) ProcessPayment(transactionID string, bankID uint32, aid string, installments uint32, useLoyalty bool) (*pb.PaymentCompletionResponse, *pb.LoyaltyCardInquiryResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_PaymentProcessingRequest{
			PaymentProcessingRequest: &pb.PaymentProcessingRequest{
				SelectedBankId:   bankID,
				UseLoyaltyPoints: useLoyalty,
				TransactionId:    transactionID,
				SelectedAid:      aid,
				InstallmentCount: installments,
			},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, nil, err
	}

	// Check if we got loyalty inquiry response
	if resp, ok := response.Body.(*pb.Message_LoyaltyCardInquiryResponse); ok {
		return nil, resp.LoyaltyCardInquiryResponse, nil
	}

	// Check if we got payment completion response
	if resp, ok := response.Body.(*pb.Message_PaymentCompletionResponse); ok {
		if resp.PaymentCompletionResponse.Code != "00" {
			return resp.PaymentCompletionResponse, nil, fmt.Errorf("payment failed: %s", resp.PaymentCompletionResponse.Message)
		}
		return resp.PaymentCompletionResponse, nil, nil
	}

	return nil, nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// ConfirmLoyaltyPoints confirms loyalty points usage
func (api *API) ConfirmLoyaltyPoints(transactionID string, pointsToUse, pointsValue uint32) (*pb.PaymentCompletionResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_LoyaltyPointsConfirmation{
			LoyaltyPointsConfirmation: &pb.LoyaltyPointsConfirmation{
				PointsToUse:      pointsToUse,
				PointsValueCents: pointsValue,
				TransactionId:    transactionID,
			},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	if resp, ok := response.Body.(*pb.Message_PaymentCompletionResponse); ok {
		if resp.PaymentCompletionResponse.Code != "00" {
			return resp.PaymentCompletionResponse, fmt.Errorf("payment failed: %s", resp.PaymentCompletionResponse.Message)
		}
		return resp.PaymentCompletionResponse, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// RefundTransaction refunds a transaction
func (api *API) RefundTransaction(originalTxnID, originalConfCode string, refundAmount uint32, currency, reason string) (*pb.RefundTransactionResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_RefundTransactionRequest{
			RefundTransactionRequest: &pb.RefundTransactionRequest{
				OriginalTransactionId:    originalTxnID,
				RefundAmountCents:        refundAmount,
				Reason:                   reason,
				Currency:                 currency,
				OriginalConfirmationCode: originalConfCode,
			},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	if resp, ok := response.Body.(*pb.Message_RefundTransactionResponse); ok {
		if resp.RefundTransactionResponse.Code != "00" {
			return resp.RefundTransactionResponse, fmt.Errorf("refund failed: %s", resp.RefundTransactionResponse.Message)
		}
		return resp.RefundTransactionResponse, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// VoidTransaction voids a transaction
func (api *API) VoidTransaction(originalTxnID, originalConfCode, currency, reason string) (*pb.VoidTransactionResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_VoidTransactionRequest{
			VoidTransactionRequest: &pb.VoidTransactionRequest{
				OriginalTransactionId:    originalTxnID,
				Reason:                   reason,
				Currency:                 currency,
				OriginalConfirmationCode: originalConfCode,
			},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	if resp, ok := response.Body.(*pb.Message_VoidTransactionResponse); ok {
		if resp.VoidTransactionResponse.Code != "00" {
			return resp.VoidTransactionResponse, fmt.Errorf("void failed: %s", resp.VoidTransactionResponse.Message)
		}
		return resp.VoidTransactionResponse, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// GetXReport retrieves an X report
func (api *API) GetXReport() (*pb.XReportResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_XReportRequest{
			XReportRequest: &pb.XReportRequest{},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	if resp, ok := response.Body.(*pb.Message_XReportResponse); ok {
		return resp.XReportResponse, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// GetZReport retrieves a Z report
func (api *API) GetZReport() (*pb.ZReportResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_ZReportRequest{
			ZReportRequest: &pb.ZReportRequest{},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	if resp, ok := response.Body.(*pb.Message_ZReportResponse); ok {
		return resp.ZReportResponse, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// GetDetailedReport retrieves a detailed transaction report
func (api *API) GetDetailedReport(fromTime, toTime string, limit uint32, includeVoids bool, currency string) (*pb.DetailedReportResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_DetailedReportRequest{
			DetailedReportRequest: &pb.DetailedReportRequest{
				FromTimestamp: fromTime,
				ToTimestamp:   toTime,
				Limit:         limit,
				IncludeVoids:  includeVoids,
				Currency:      currency,
			},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	if resp, ok := response.Body.(*pb.Message_DetailedReportResponse); ok {
		return resp.DetailedReportResponse, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// InquireGiftCard inquires gift card balance
func (api *API) InquireGiftCard(cardNumber, currency string) (*pb.GiftCardInquiryResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GiftCardInquiryRequest{
			GiftCardInquiryRequest: &pb.GiftCardInquiryRequest{
				CardNumber: cardNumber,
				Currency:   currency,
			},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	if resp, ok := response.Body.(*pb.Message_GiftCardInquiryResponse); ok {
		return resp.GiftCardInquiryResponse, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// ChargeGiftCard charges a gift card
func (api *API) ChargeGiftCard(cardNumber string, amountCents, transactionAmount uint32, currency string) (*pb.GiftCardChargeResponse, error) {
	msg := &pb.Message{
		MessageId: generateMessageID(),
		Timestamp: time.Now().Format(time.RFC3339),
		Body: &pb.Message_GiftCardChargeRequest{
			GiftCardChargeRequest: &pb.GiftCardChargeRequest{
				CardNumber:             cardNumber,
				AmountCents:            amountCents,
				TransactionAmountCents: transactionAmount,
				Currency:               currency,
			},
		},
	}

	response, err := api.client.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	if resp, ok := response.Body.(*pb.Message_GiftCardChargeResponse); ok {
		return resp.GiftCardChargeResponse, nil
	}

	return nil, fmt.Errorf("unexpected response type: %T", response.Body)
}

// Ping sends a ping to the POS terminal
func (api *API) Ping() error {
	return api.client.Ping()
}

// GetClient returns the underlying client
func (api *API) GetClient() *Client {
	return api.client
}

// generateMessageID generates a unique message ID
func generateMessageID() string {
	return fmt.Sprintf("ECR-%d", time.Now().UnixNano())
}
