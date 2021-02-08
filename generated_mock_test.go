// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/jackc/pgx/v4 (interfaces: Tx,Rows)

// Package pgmig is a generated GoMock package.
package pgmig

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	pgconn "github.com/jackc/pgconn"
	v2 "github.com/jackc/pgproto3/v2"
	v4 "github.com/jackc/pgx/v4"
	reflect "reflect"
)

// MockTx is a mock of Tx interface
type MockTx struct {
	ctrl     *gomock.Controller
	recorder *MockTxMockRecorder
}

// MockTxMockRecorder is the mock recorder for MockTx
type MockTxMockRecorder struct {
	mock *MockTx
}

// NewMockTx creates a new mock instance
func NewMockTx(ctrl *gomock.Controller) *MockTx {
	mock := &MockTx{ctrl: ctrl}
	mock.recorder = &MockTxMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockTx) EXPECT() *MockTxMockRecorder {
	return m.recorder
}

// Begin mocks base method
func (m *MockTx) Begin(arg0 context.Context) (v4.Tx, error) {
	ret := m.ctrl.Call(m, "Begin", arg0)
	ret0, _ := ret[0].(v4.Tx)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Begin indicates an expected call of Begin
func (mr *MockTxMockRecorder) Begin(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Begin", reflect.TypeOf((*MockTx)(nil).Begin), arg0)
}

// Commit mocks base method
func (m *MockTx) Commit(arg0 context.Context) error {
	ret := m.ctrl.Call(m, "Commit", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Commit indicates an expected call of Commit
func (mr *MockTxMockRecorder) Commit(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Commit", reflect.TypeOf((*MockTx)(nil).Commit), arg0)
}

// Conn mocks base method
func (m *MockTx) Conn() *v4.Conn {
	ret := m.ctrl.Call(m, "Conn")
	ret0, _ := ret[0].(*v4.Conn)
	return ret0
}

// Conn indicates an expected call of Conn
func (mr *MockTxMockRecorder) Conn() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Conn", reflect.TypeOf((*MockTx)(nil).Conn))
}

// CopyFrom mocks base method
func (m *MockTx) CopyFrom(arg0 context.Context, arg1 v4.Identifier, arg2 []string, arg3 v4.CopyFromSource) (int64, error) {
	ret := m.ctrl.Call(m, "CopyFrom", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CopyFrom indicates an expected call of CopyFrom
func (mr *MockTxMockRecorder) CopyFrom(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CopyFrom", reflect.TypeOf((*MockTx)(nil).CopyFrom), arg0, arg1, arg2, arg3)
}

// Exec mocks base method
func (m *MockTx) Exec(arg0 context.Context, arg1 string, arg2 ...interface{}) (pgconn.CommandTag, error) {
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Exec", varargs...)
	ret0, _ := ret[0].(pgconn.CommandTag)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Exec indicates an expected call of Exec
func (mr *MockTxMockRecorder) Exec(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Exec", reflect.TypeOf((*MockTx)(nil).Exec), varargs...)
}

// LargeObjects mocks base method
func (m *MockTx) LargeObjects() v4.LargeObjects {
	ret := m.ctrl.Call(m, "LargeObjects")
	ret0, _ := ret[0].(v4.LargeObjects)
	return ret0
}

// LargeObjects indicates an expected call of LargeObjects
func (mr *MockTxMockRecorder) LargeObjects() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LargeObjects", reflect.TypeOf((*MockTx)(nil).LargeObjects))
}

// Prepare mocks base method
func (m *MockTx) Prepare(arg0 context.Context, arg1, arg2 string) (*pgconn.StatementDescription, error) {
	ret := m.ctrl.Call(m, "Prepare", arg0, arg1, arg2)
	ret0, _ := ret[0].(*pgconn.StatementDescription)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Prepare indicates an expected call of Prepare
func (mr *MockTxMockRecorder) Prepare(arg0, arg1, arg2 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Prepare", reflect.TypeOf((*MockTx)(nil).Prepare), arg0, arg1, arg2)
}

// Query mocks base method
func (m *MockTx) Query(arg0 context.Context, arg1 string, arg2 ...interface{}) (v4.Rows, error) {
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Query", varargs...)
	ret0, _ := ret[0].(v4.Rows)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Query indicates an expected call of Query
func (mr *MockTxMockRecorder) Query(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Query", reflect.TypeOf((*MockTx)(nil).Query), varargs...)
}

// QueryRow mocks base method
func (m *MockTx) QueryRow(arg0 context.Context, arg1 string, arg2 ...interface{}) v4.Row {
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "QueryRow", varargs...)
	ret0, _ := ret[0].(v4.Row)
	return ret0
}

// QueryRow indicates an expected call of QueryRow
func (mr *MockTxMockRecorder) QueryRow(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryRow", reflect.TypeOf((*MockTx)(nil).QueryRow), varargs...)
}

// Rollback mocks base method
func (m *MockTx) Rollback(arg0 context.Context) error {
	ret := m.ctrl.Call(m, "Rollback", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Rollback indicates an expected call of Rollback
func (mr *MockTxMockRecorder) Rollback(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Rollback", reflect.TypeOf((*MockTx)(nil).Rollback), arg0)
}

// SendBatch mocks base method
func (m *MockTx) SendBatch(arg0 context.Context, arg1 *v4.Batch) v4.BatchResults {
	ret := m.ctrl.Call(m, "SendBatch", arg0, arg1)
	ret0, _ := ret[0].(v4.BatchResults)
	return ret0
}

// SendBatch indicates an expected call of SendBatch
func (mr *MockTxMockRecorder) SendBatch(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendBatch", reflect.TypeOf((*MockTx)(nil).SendBatch), arg0, arg1)
}

// MockRows is a mock of Rows interface
type MockRows struct {
	ctrl     *gomock.Controller
	recorder *MockRowsMockRecorder
}

// MockRowsMockRecorder is the mock recorder for MockRows
type MockRowsMockRecorder struct {
	mock *MockRows
}

// NewMockRows creates a new mock instance
func NewMockRows(ctrl *gomock.Controller) *MockRows {
	mock := &MockRows{ctrl: ctrl}
	mock.recorder = &MockRowsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockRows) EXPECT() *MockRowsMockRecorder {
	return m.recorder
}

// Close mocks base method
func (m *MockRows) Close() {
	m.ctrl.Call(m, "Close")
}

// Close indicates an expected call of Close
func (mr *MockRowsMockRecorder) Close() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockRows)(nil).Close))
}

// CommandTag mocks base method
func (m *MockRows) CommandTag() pgconn.CommandTag {
	ret := m.ctrl.Call(m, "CommandTag")
	ret0, _ := ret[0].(pgconn.CommandTag)
	return ret0
}

// CommandTag indicates an expected call of CommandTag
func (mr *MockRowsMockRecorder) CommandTag() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommandTag", reflect.TypeOf((*MockRows)(nil).CommandTag))
}

// Err mocks base method
func (m *MockRows) Err() error {
	ret := m.ctrl.Call(m, "Err")
	ret0, _ := ret[0].(error)
	return ret0
}

// Err indicates an expected call of Err
func (mr *MockRowsMockRecorder) Err() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Err", reflect.TypeOf((*MockRows)(nil).Err))
}

// FieldDescriptions mocks base method
func (m *MockRows) FieldDescriptions() []v2.FieldDescription {
	ret := m.ctrl.Call(m, "FieldDescriptions")
	ret0, _ := ret[0].([]v2.FieldDescription)
	return ret0
}

// FieldDescriptions indicates an expected call of FieldDescriptions
func (mr *MockRowsMockRecorder) FieldDescriptions() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FieldDescriptions", reflect.TypeOf((*MockRows)(nil).FieldDescriptions))
}

// Next mocks base method
func (m *MockRows) Next() bool {
	ret := m.ctrl.Call(m, "Next")
	ret0, _ := ret[0].(bool)
	return ret0
}

// Next indicates an expected call of Next
func (mr *MockRowsMockRecorder) Next() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Next", reflect.TypeOf((*MockRows)(nil).Next))
}

// RawValues mocks base method
func (m *MockRows) RawValues() [][]byte {
	ret := m.ctrl.Call(m, "RawValues")
	ret0, _ := ret[0].([][]byte)
	return ret0
}

// RawValues indicates an expected call of RawValues
func (mr *MockRowsMockRecorder) RawValues() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RawValues", reflect.TypeOf((*MockRows)(nil).RawValues))
}

// Scan mocks base method
func (m *MockRows) Scan(arg0 ...interface{}) error {
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Scan", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Scan indicates an expected call of Scan
func (mr *MockRowsMockRecorder) Scan(arg0 ...interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Scan", reflect.TypeOf((*MockRows)(nil).Scan), arg0...)
}

// Values mocks base method
func (m *MockRows) Values() ([]interface{}, error) {
	ret := m.ctrl.Call(m, "Values")
	ret0, _ := ret[0].([]interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Values indicates an expected call of Values
func (mr *MockRowsMockRecorder) Values() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Values", reflect.TypeOf((*MockRows)(nil).Values))
}
