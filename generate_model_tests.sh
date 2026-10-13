#!/bin/bash
cat << 'GO_EOF' > tests/models_full_test.go
package tests

import (
	"testing"
	paymentmanager "github.com/veltylabs/payment_manager"
	"webtyp.com/model"
)

type dummyWriter struct{}
func (w dummyWriter) Object(name string) model.FieldWriter { return w }
func (w dummyWriter) Array(name string, len int) model.ArrayWriter { return w }
func (w dummyWriter) Next() model.FieldWriter { return w }
func (w dummyWriter) Null(name string) {}
func (w dummyWriter) String(name string, v string) {}
func (w dummyWriter) Int64(name string, v int64) {}
func (w dummyWriter) Bool(name string, v bool) {}
func (w dummyWriter) Uint32(name string, v uint32) {}
func (w dummyWriter) Int32(name string, v int32) {}
func (w dummyWriter) Float64(name string, v float64) {}
func (w dummyWriter) Bytes(name string, v []byte) {}

type dummyReader struct{}
func (r dummyReader) Object() model.FieldReader { return r }
func (r dummyReader) Array() model.ArrayReader { return r }
func (r dummyReader) Next() model.FieldReader { return r }
func (r dummyReader) Len() int { return 0 }
func (r dummyReader) Kind() model.Kind { return model.KindNull }
func (r dummyReader) String() string { return "" }
func (r dummyReader) Int64() int64 { return 0 }
func (r dummyReader) Bool() bool { return false }
func (r dummyReader) Uint32() uint32 { return 0 }
func (r dummyReader) Int32() int32 { return 0 }
func (r dummyReader) Float64() float64 { return 0 }
func (r dummyReader) Bytes() []byte { return nil }
func (r dummyReader) NextField() string { return "" }

func TestModels_All(t *testing.T) {
	models := []model.Model{
		&paymentmanager.Payment{},
		&paymentmanager.Refund{},
		&paymentmanager.GetChargeArgs{},
		&paymentmanager.CreateCounterPaymentArgs{},
		&paymentmanager.StartOnlinePaymentArgs{},
		&paymentmanager.GetPaymentArgs{},
		&paymentmanager.ListPaymentsByReferenceArgs{},
		&paymentmanager.ListPaymentsArgs{},
		&paymentmanager.RefundPaymentArgs{},
		&paymentmanager.ListRefundsArgs{},
		&paymentmanager.ReconcileOnlinePaymentsArgs{},
	}
	for _, m := range models {
		m.Validate(byte(model.ActionCreate))
		m.Validate(byte(model.ActionRead))
		m.Validate(byte(model.ActionUpdate))
		m.ModelName()
		m.Schema()
		m.Pointers()
		m.IsNil()
		w := dummyWriter{}
		m.EncodeFields(w)
		r := dummyReader{}
		m.DecodeFields(r)
	}
}
GO_EOF
