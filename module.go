package paymentmanager

import (
	"webtyp.com/events"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/storage"
	"webtyp.com/time"
)

const (
	EventPaymentPaid     = "payment.paid"
	EventPaymentFailed   = "payment.failed"
	EventPaymentRefunded = "payment.refunded"
)

type Deps struct {
	Publisher events.Publisher
	Gateway   Gateway
	ReturnURL string
	Charges   []ChargeSource
}

type Module struct {
	db      *orm.DB
	repo    *repository
	pub     events.Publisher
	gw      Gateway
	retURL  string
	charges []ChargeSource
}

func New(db *orm.DB, deps Deps) (*Module, error) {
	if len(deps.Charges) == 0 {
		return nil, fmt.Err("payment_manager: Deps.Charges is required")
	}
	if deps.Gateway != nil && deps.ReturnURL == "" {
		return nil, fmt.Err("payment_manager: Deps.ReturnURL is required with a Gateway")
	}
	return &Module{
		db:      db,
		repo:    &repository{db: db},
		pub:     deps.Publisher,
		gw:      deps.Gateway,
		retURL:  deps.ReturnURL,
		charges: deps.Charges,
	}, nil
}

func (m *Module) DB() *orm.DB {
	return m.db
}

func (m *Module) publish(topic string, payload model.Encodable) {
	if m.pub != nil {
		m.pub.Publish(events.Event{Topic: topic, Payload: payload})
	}
}

func (m *Module) Charge(tenantId, referenceModule, referenceId string) (amount int64, currency string, payable bool, err error) {
	var reader ChargeReader
	for _, c := range m.charges {
		if c.Module == referenceModule {
			reader = c.Reader
			break
		}
	}
	if reader == nil {
		return 0, "", false, ErrUnknownReference
	}

	amt, cur, pay, err := reader.Charge(tenantId, referenceId)
	if err != nil {
		return 0, "", false, err
	}

	if cur != CurrencyCLP && cur != CurrencyUSD {
		return 0, "", false, ErrUnsupportedCurrency
	}

	return amt, cur, pay, nil
}

func (m *Module) CreateCounterPayment(cmd CreateCounterPaymentArgs) (Payment, error) {
	if cmd.TenantId == "" || cmd.Id == "" || cmd.ReferenceModule == "" || cmd.ReferenceId == "" || cmd.Method == "" {
		return Payment{}, ErrMissingArgs
	}

	existing, err := m.repo.paymentById(cmd.TenantId, cmd.Id)
	if err == nil {
		if existing.ReferenceModule == cmd.ReferenceModule && existing.ReferenceId == cmd.ReferenceId {
			return existing, nil
		}
		return Payment{}, ErrIdTaken
	}
	if err != ErrNotFound {
		return Payment{}, err
	}

	if cmd.Method != MethodCash && cmd.Method != MethodDebit && cmd.Method != MethodCredit && cmd.Method != MethodTransfer {
		return Payment{}, ErrInvalidMethod
	}

	payments, err := m.repo.paymentsByReference(cmd.TenantId, cmd.ReferenceModule, cmd.ReferenceId)
	if err != nil {
		return Payment{}, err
	}
	for _, p := range payments {
		if p.Status == StatusPaid || p.Status == StatusPartiallyRefunded {
			return Payment{}, ErrAlreadyPaid
		}
	}

	amt, cur, payable, err := m.Charge(cmd.TenantId, cmd.ReferenceModule, cmd.ReferenceId)
	if err != nil {
		return Payment{}, err
	}
	if !payable {
		return Payment{}, ErrNotPayable
	}

	now := time.Now() / 1_000_000_000
	p := Payment{
		Id:              cmd.Id,
		TenantId:        cmd.TenantId,
		ReferenceModule: cmd.ReferenceModule,
		ReferenceId:     cmd.ReferenceId,
		Channel:         ChannelCounter,
		Method:          cmd.Method,
		Amount:          amt,
		Currency:        cur,
		Status:          StatusPaid,
		ReceiptNumber:   cmd.ReceiptNumber,
		CreatedAt:       now,
		PaidAt:          now,
		Revision:        1,
	}

	if err := p.Validate(byte(model.ActionCreate)); err != nil {
		return Payment{}, err
	}

	if err := m.db.Create(&p); err != nil {
		return Payment{}, err
	}

	m.publish(EventPaymentPaid, &p)
	return p, nil
}

func (m *Module) StartOnlinePayment(cmd StartOnlinePaymentArgs) (StartOnlinePaymentResult, error) {
	if cmd.TenantId == "" || cmd.Id == "" || cmd.ReferenceModule == "" || cmd.ReferenceId == "" || cmd.ReturnTo == "" {
		return StartOnlinePaymentResult{}, ErrMissingArgs
	}

	if m.gw == nil {
		return StartOnlinePaymentResult{}, ErrGatewayUnavailable
	}

	_, err := m.repo.paymentById(cmd.TenantId, cmd.Id)
	if err == nil {
		return StartOnlinePaymentResult{}, ErrIdTaken
	}
	if err != ErrNotFound {
		return StartOnlinePaymentResult{}, err
	}

	payments, err := m.repo.paymentsByReference(cmd.TenantId, cmd.ReferenceModule, cmd.ReferenceId)
	if err != nil {
		return StartOnlinePaymentResult{}, err
	}
	for _, p := range payments {
		if p.Status == StatusPaid || p.Status == StatusPartiallyRefunded {
			return StartOnlinePaymentResult{}, ErrAlreadyPaid
		}
	}

	amt, cur, payable, err := m.Charge(cmd.TenantId, cmd.ReferenceModule, cmd.ReferenceId)
	if err != nil {
		return StartOnlinePaymentResult{}, err
	}
	if !payable {
		return StartOnlinePaymentResult{}, ErrNotPayable
	}

	now := time.Now() / 1_000_000_000
	p := Payment{
		Id:              cmd.Id,
		TenantId:        cmd.TenantId,
		ReferenceModule: cmd.ReferenceModule,
		ReferenceId:     cmd.ReferenceId,
		Channel:         ChannelOnline,
		Method:          MethodWebpay,
		Amount:          amt,
		Currency:        cur,
		Status:          StatusPending,
		ReturnTo:        cmd.ReturnTo,
		CreatedAt:       now,
		Revision:        1,
	}

	if err := p.Validate(byte(model.ActionCreate)); err != nil {
		return StartOnlinePaymentResult{}, err
	}

	res, gwErr := m.gw.Start(GatewayStart{
		BuyOrder:  cmd.Id,
		SessionId: cmd.Id,
		Amount:    amt,
		Currency:  cur,
		ReturnURL: m.retURL,
	})

	if gwErr != nil {
		p.Status = StatusFailed
		if err := m.db.Create(&p); err != nil {
			return StartOnlinePaymentResult{}, err
		}
		return StartOnlinePaymentResult{}, ErrGatewayFailed
	}

	p.ProviderToken = res.Token
	if err := m.db.Create(&p); err != nil {
		return StartOnlinePaymentResult{}, err
	}

	return StartOnlinePaymentResult{
		PaymentId:   p.Id,
		RedirectUrl: res.RedirectURL,
		Token:       res.Token,
	}, nil
}

func (m *Module) CompleteOnlinePayment(ret OnlineReturn) (Payment, error) {
	var p Payment
	var err error

	if ret.Token != "" && ret.AbortToken == "" {
		p, err = m.repo.paymentByProviderToken(ret.Token)
	} else if ret.AbortToken != "" {
		p, err = m.repo.paymentByProviderToken(ret.AbortToken)
	} else if ret.BuyOrder != "" {
		err = m.db.Query(&p).Where("id").Eq(ret.BuyOrder).ReadOne()
		if err != nil && orm.IsNotFound(err) {
			err = ErrNotFound
		}
	} else {
		return Payment{}, ErrMissingArgs
	}

	if err != nil {
		return Payment{}, err
	}

	if p.Status != StatusPending {
		return p, nil
	}

	var nextStatus string
	var errRet error

	if ret.Token != "" && ret.AbortToken == "" {
		gwRes, gwErr := m.gw.Commit(ret.Token)
		if gwErr != nil {
			return p, ErrGatewayFailed
		}

		if gwRes.Outcome == OutcomeAuthorized {
			if gwRes.Amount == p.Amount {
				nextStatus = StatusPaid
				p.AuthorizationCode = gwRes.AuthorizationCode
				p.PaymentTypeCode = gwRes.PaymentTypeCode
				p.Installments = gwRes.Installments
				p.CardLast4 = gwRes.CardLast4
				p.PaidAt = time.Now() / 1_000_000_000
			} else {
				m.gw.Refund(ret.Token, gwRes.Amount)
				nextStatus = StatusFailed
				errRet = ErrAmountMismatch
			}
		} else if gwRes.Outcome == OutcomeRejected {
			nextStatus = StatusFailed
		} else {
			return p, nil
		}
	} else if ret.AbortToken != "" {
		nextStatus = StatusCancelled
	} else if ret.BuyOrder != "" {
		nextStatus = StatusExpired
	}

	p.Status = nextStatus
	p.Revision++

	if valErr := p.Validate(byte(model.ActionUpdate)); valErr != nil {
		return Payment{}, valErr
	}

	updateErr := m.db.Update(&p,
		storage.Eq("tenant_id", p.TenantId),
		storage.Eq("id", p.Id),
		storage.Eq("revision", p.Revision-1),
	)

	if updateErr != nil {
		if orm.IsNotFound(updateErr) {
			return Payment{}, ErrConflict
		}
		return Payment{}, updateErr
	}

	if nextStatus == StatusPaid {
		m.publish(EventPaymentPaid, &p)
	} else if nextStatus == StatusFailed {
		m.publish(EventPaymentFailed, &p)
	}

	if errRet != nil {
		return p, errRet
	}

	return p, nil
}

func (m *Module) RefundPayment(cmd RefundPaymentArgs) (Refund, error) {
	if cmd.TenantId == "" || cmd.Id == "" || cmd.PaymentId == "" || cmd.Amount <= 0 || cmd.Reason == "" {
		return Refund{}, ErrMissingArgs
	}

	existing, err := m.repo.refundById(cmd.TenantId, cmd.Id)
	if err == nil {
		if existing.PaymentId == cmd.PaymentId {
			return existing, nil
		}
		return Refund{}, ErrIdTaken
	}
	if err != ErrRefundNotFound {
		return Refund{}, err
	}

	p, err := m.repo.paymentById(cmd.TenantId, cmd.PaymentId)
	if err != nil {
		return Refund{}, err
	}

	if p.Status != StatusPaid && p.Status != StatusPartiallyRefunded {
		return Refund{}, ErrInvalidTransition
	}

	if cmd.Amount > p.Amount-p.RefundedAmount {
		return Refund{}, ErrRefundExceeds
	}

	now := time.Now() / 1_000_000_000
	ref := Refund{
		Id:        cmd.Id,
		TenantId:  cmd.TenantId,
		PaymentId: cmd.PaymentId,
		Amount:    cmd.Amount,
		Reason:    cmd.Reason,
		CreatedAt: now,
	}

	if p.Channel == ChannelOnline {
		gwRes, gwErr := m.gw.Refund(p.ProviderToken, cmd.Amount)
		if gwErr != nil {
			return Refund{}, ErrGatewayFailed
		}
		ref.Kind = gwRes.Kind
		ref.AuthorizationCode = gwRes.AuthorizationCode
		if gwRes.Amount != 0 {
			ref.Amount = gwRes.Amount
		}
	} else {
		if cmd.Method != MethodCash && cmd.Method != MethodDebit && cmd.Method != MethodCredit && cmd.Method != MethodTransfer {
			return Refund{}, ErrInvalidMethod
		}
		ref.Kind = RefundKindManual
		ref.Method = cmd.Method
	}

	if err := ref.Validate(byte(model.ActionCreate)); err != nil {
		return Refund{}, err
	}

	p.RefundedAmount += ref.Amount
	var nextStatus string
	if p.RefundedAmount == p.Amount {
		nextStatus = StatusRefunded
	} else {
		nextStatus = StatusPartiallyRefunded
	}

	prevRevision := p.Revision
	p.Status = nextStatus
	p.Revision++

	if err := p.Validate(byte(model.ActionUpdate)); err != nil {
		return Refund{}, err
	}

	txErr := m.db.Tx(func(txDB *orm.DB) error {
		if err := txDB.Create(&ref); err != nil {
			return err
		}

		updateErr := txDB.Update(&p,
			storage.Eq("tenant_id", p.TenantId),
			storage.Eq("id", p.Id),
			storage.Eq("revision", prevRevision),
		)
		if updateErr != nil {
			if orm.IsNotFound(updateErr) {
				return ErrConflict
			}
			return updateErr
		}
		return nil
	})

	if txErr != nil {
		return Refund{}, txErr
	}

	m.publish(EventPaymentRefunded, &ref)
	return ref, nil
}

func (m *Module) ReconcileOnlinePayments(tenantId string, now int64) (ReconcileResult, error) {
	var closed int

	payments, err := m.repo.oldPendingOnlinePayments(tenantId, now-OnlinePendingTimeout)
	if err != nil {
		return ReconcileResult{}, err
	}

	for _, p := range payments {
		if p.CreatedAt < now-GatewayStatusWindow {
			p.Status = StatusExpired
			p.Revision++
			updateErr := m.db.Update(p,
				storage.Eq("tenant_id", p.TenantId),
				storage.Eq("id", p.Id),
				storage.Eq("revision", p.Revision-1),
			)
			if updateErr == nil {
				closed++
			}
			continue
		}

		gwRes, gwErr := m.gw.Status(p.ProviderToken)
		if gwErr != nil {
			continue // saltarlo y seguir
		}

		var nextStatus string
		if gwRes.Outcome == OutcomeAuthorized {
			if gwRes.Amount == p.Amount {
				nextStatus = StatusPaid
				p.AuthorizationCode = gwRes.AuthorizationCode
				p.PaymentTypeCode = gwRes.PaymentTypeCode
				p.Installments = gwRes.Installments
				p.CardLast4 = gwRes.CardLast4
				p.PaidAt = now
			} else {
				m.gw.Refund(p.ProviderToken, gwRes.Amount)
				nextStatus = StatusFailed
			}
		} else if gwRes.Outcome == OutcomeRejected {
			nextStatus = StatusFailed
		} else if gwRes.Outcome == OutcomePending {
			nextStatus = StatusExpired
		} else {
			continue
		}

		p.Status = nextStatus
		p.Revision++

		updateErr := m.db.Update(p,
			storage.Eq("tenant_id", p.TenantId),
			storage.Eq("id", p.Id),
			storage.Eq("revision", p.Revision-1),
		)

		if updateErr == nil {
			closed++
			if nextStatus == StatusPaid {
				m.publish(EventPaymentPaid, p)
			} else if nextStatus == StatusFailed {
				m.publish(EventPaymentFailed, p)
			}
		}
	}

	return ReconcileResult{Closed: int64(closed)}, nil
}

func (m *Module) PaidAmount(tenantId, paymentId, referenceModule, referenceId string) (amount int64, currency string, err error) {
	p, err := m.repo.paymentById(tenantId, paymentId)
	if err != nil {
		return 0, "", err
	}

	if p.ReferenceModule != referenceModule || p.ReferenceId != referenceId {
		return 0, "", ErrNotFound
	}

	if p.Status != StatusPaid && p.Status != StatusPartiallyRefunded {
		return 0, "", ErrNotPaid
	}

	return p.Amount - p.RefundedAmount, p.Currency, nil
}

func (m *Module) GetPayment(args GetPaymentArgs) (Payment, error) {
	return m.repo.paymentById(args.TenantId, args.Id)
}

func (m *Module) ListPaymentsByReference(args ListPaymentsByReferenceArgs) ([]*Payment, error) {
	return m.repo.paymentsByReference(args.TenantId, args.ReferenceModule, args.ReferenceId)
}

func (m *Module) ListPayments(args ListPaymentsArgs) ([]*Payment, error) {
	return m.repo.paymentsByDateRange(args.TenantId, args.From, args.To)
}

func (m *Module) ListRefunds(args ListRefundsArgs) ([]*Refund, error) {
	return m.repo.refundsByPayment(args.TenantId, args.PaymentId)
}
