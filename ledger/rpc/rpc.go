package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Glide28/go_hw_10/ledger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

const serviceName = "cashcraft.Ledger"

type jsonCodec struct{}

func (jsonCodec) Name() string                           { return "json" }
func (jsonCodec) Marshal(value any) ([]byte, error)      { return json.Marshal(value) }
func (jsonCodec) Unmarshal(data []byte, value any) error { return json.Unmarshal(data, value) }
func init()                                              { encoding.RegisterCodec(jsonCodec{}) }

type empty struct{}
type budgetList struct {
	Items []ledger.Budget `json:"items"`
}
type transactionList struct {
	Items []ledger.Transaction `json:"items"`
}
type bulkImportRequest struct {
	Items   []ledger.Transaction `json:"items"`
	Workers int                  `json:"workers"`
}
type reportRequest struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}
type reportList struct {
	Items []ledger.ReportItem `json:"items"`
}

type Server struct{ service ledger.Service }
type serverInterface interface{}

func Register(server *grpc.Server, service ledger.Service) {
	server.RegisterService(&serviceDescription, &Server{service: service})
}
func (s *Server) setBudget(ctx context.Context, request *ledger.Budget) (*ledger.Budget, error) {
	value, err := s.service.SetBudget(ctx, *request)
	return &value, mapServerError(err)
}
func (s *Server) listBudgets(ctx context.Context, _ *empty) (*budgetList, error) {
	items, err := s.service.ListBudgets(ctx)
	return &budgetList{Items: items}, mapServerError(err)
}
func (s *Server) addTransaction(ctx context.Context, request *ledger.Transaction) (*ledger.Transaction, error) {
	value, err := s.service.AddTransaction(ctx, *request)
	return &value, mapServerError(err)
}
func (s *Server) bulkImport(ctx context.Context, request *bulkImportRequest) (*ledger.ImportSummary, error) {
	value, err := s.service.BulkImport(ctx, request.Items, request.Workers)
	return &value, mapServerError(err)
}
func (s *Server) listTransactions(ctx context.Context, _ *empty) (*transactionList, error) {
	items, err := s.service.ListTransactions(ctx)
	return &transactionList{Items: items}, mapServerError(err)
}
func (s *Server) report(ctx context.Context, request *reportRequest) (*reportList, error) {
	items, err := s.service.GetReportSummary(ctx, request.From, request.To)
	return &reportList{Items: items}, mapServerError(err)
}

func mapServerError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ledger.ErrBudgetExceeded) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

type Client struct{ connection *grpc.ClientConn }

func Dial(address string) (*Client, error) {
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})))
	if err != nil {
		return nil, err
	}
	return &Client{connection: connection}, nil
}
func (c *Client) Close() error { return c.connection.Close() }
func (c *Client) SetBudget(ctx context.Context, value ledger.Budget) (ledger.Budget, error) {
	var response ledger.Budget
	err := c.connection.Invoke(ctx, "/"+serviceName+"/SetBudget", &value, &response)
	return response, mapClientError(err)
}
func (c *Client) ListBudgets(ctx context.Context) ([]ledger.Budget, error) {
	var response budgetList
	err := c.connection.Invoke(ctx, "/"+serviceName+"/ListBudgets", &empty{}, &response)
	return response.Items, mapClientError(err)
}
func (c *Client) AddTransaction(ctx context.Context, value ledger.Transaction) (ledger.Transaction, error) {
	var response ledger.Transaction
	err := c.connection.Invoke(ctx, "/"+serviceName+"/AddTransaction", &value, &response)
	return response, mapClientError(err)
}
func (c *Client) BulkImport(ctx context.Context, items []ledger.Transaction, workers int) (ledger.ImportSummary, error) {
	var response ledger.ImportSummary
	err := c.connection.Invoke(ctx, "/"+serviceName+"/BulkImport", &bulkImportRequest{Items: items, Workers: workers}, &response)
	return response, mapClientError(err)
}
func (c *Client) ListTransactions(ctx context.Context) ([]ledger.Transaction, error) {
	var response transactionList
	err := c.connection.Invoke(ctx, "/"+serviceName+"/ListTransactions", &empty{}, &response)
	return response.Items, mapClientError(err)
}
func (c *Client) GetReportSummary(ctx context.Context, from, to time.Time) ([]ledger.ReportItem, error) {
	var response reportList
	err := c.connection.Invoke(ctx, "/"+serviceName+"/GetReportSummary", &reportRequest{From: from, To: to}, &response)
	return response.Items, mapClientError(err)
}
func mapClientError(err error) error {
	if err == nil {
		return nil
	}
	switch status.Code(err) {
	case codes.FailedPrecondition:
		return ledger.ErrBudgetExceeded
	case codes.Canceled:
		return context.Canceled
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	default:
		return err
	}
}

func unaryHandler[Request any, Response any](method string, call func(*Server, context.Context, *Request) (*Response, error)) grpc.MethodDesc {
	return grpc.MethodDesc{MethodName: method, Handler: func(server any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		request := new(Request)
		if err := decode(request); err != nil {
			return nil, err
		}
		invoke := func(ctx context.Context, request any) (any, error) {
			return call(server.(*Server), ctx, request.(*Request))
		}
		if interceptor == nil {
			return invoke(ctx, request)
		}
		return interceptor(ctx, request, &grpc.UnaryServerInfo{Server: server, FullMethod: "/" + serviceName + "/" + method}, invoke)
	}}
}

var serviceDescription = grpc.ServiceDesc{
	ServiceName: serviceName,
	HandlerType: (*serverInterface)(nil),
	Methods: []grpc.MethodDesc{
		unaryHandler("SetBudget", (*Server).setBudget),
		unaryHandler("ListBudgets", (*Server).listBudgets),
		unaryHandler("AddTransaction", (*Server).addTransaction),
		unaryHandler("BulkImport", (*Server).bulkImport),
		unaryHandler("ListTransactions", (*Server).listTransactions),
		unaryHandler("GetReportSummary", (*Server).report),
	},
}
