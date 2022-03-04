// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package inx

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// INXClient is the client API for INX service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type INXClient interface {
	// Node
	ReadNodeStatus(ctx context.Context, in *NoParams, opts ...grpc.CallOption) (*NodeStatus, error)
	ReadProtocolParameters(ctx context.Context, in *NoParams, opts ...grpc.CallOption) (*ProtocolParameters, error)
	// Milestones
	ReadMilestone(ctx context.Context, in *MilestoneRequest, opts ...grpc.CallOption) (*Milestone, error)
	ListenToLatestMilestone(ctx context.Context, in *NoParams, opts ...grpc.CallOption) (INX_ListenToLatestMilestoneClient, error)
	ListenToConfirmedMilestone(ctx context.Context, in *NoParams, opts ...grpc.CallOption) (INX_ListenToConfirmedMilestoneClient, error)
	// Messages
	ListenToMessages(ctx context.Context, in *MessageFilter, opts ...grpc.CallOption) (INX_ListenToMessagesClient, error)
	ListenToSolidMessages(ctx context.Context, in *MessageFilter, opts ...grpc.CallOption) (INX_ListenToSolidMessagesClient, error)
	ListenToReferencedMessages(ctx context.Context, in *MessageFilter, opts ...grpc.CallOption) (INX_ListenToReferencedMessagesClient, error)
	SubmitMessage(ctx context.Context, in *RawMessage, opts ...grpc.CallOption) (*MessageId, error)
	ReadMessage(ctx context.Context, in *MessageId, opts ...grpc.CallOption) (*RawMessage, error)
	ReadMessageMetadata(ctx context.Context, in *MessageId, opts ...grpc.CallOption) (*MessageMetadata, error)
	// UTXO
	ReadUnspentOutputs(ctx context.Context, in *NoParams, opts ...grpc.CallOption) (INX_ReadUnspentOutputsClient, error)
	ListenToLedgerUpdates(ctx context.Context, in *LedgerUpdateRequest, opts ...grpc.CallOption) (INX_ListenToLedgerUpdatesClient, error)
	ReadOutput(ctx context.Context, in *OutputId, opts ...grpc.CallOption) (*OutputResponse, error)
	ListenToMigrationReceipts(ctx context.Context, in *NoParams, opts ...grpc.CallOption) (INX_ListenToMigrationReceiptsClient, error)
	// REST API
	RegisterAPIRoute(ctx context.Context, in *APIRouteRequest, opts ...grpc.CallOption) (*NoParams, error)
	UnregisterAPIRoute(ctx context.Context, in *APIRouteRequest, opts ...grpc.CallOption) (*NoParams, error)
}

type iNXClient struct {
	cc grpc.ClientConnInterface
}

func NewINXClient(cc grpc.ClientConnInterface) INXClient {
	return &iNXClient{cc}
}

func (c *iNXClient) ReadNodeStatus(ctx context.Context, in *NoParams, opts ...grpc.CallOption) (*NodeStatus, error) {
	out := new(NodeStatus)
	err := c.cc.Invoke(ctx, "/inx.INX/ReadNodeStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *iNXClient) ReadProtocolParameters(ctx context.Context, in *NoParams, opts ...grpc.CallOption) (*ProtocolParameters, error) {
	out := new(ProtocolParameters)
	err := c.cc.Invoke(ctx, "/inx.INX/ReadProtocolParameters", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *iNXClient) ReadMilestone(ctx context.Context, in *MilestoneRequest, opts ...grpc.CallOption) (*Milestone, error) {
	out := new(Milestone)
	err := c.cc.Invoke(ctx, "/inx.INX/ReadMilestone", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *iNXClient) ListenToLatestMilestone(ctx context.Context, in *NoParams, opts ...grpc.CallOption) (INX_ListenToLatestMilestoneClient, error) {
	stream, err := c.cc.NewStream(ctx, &INX_ServiceDesc.Streams[0], "/inx.INX/ListenToLatestMilestone", opts...)
	if err != nil {
		return nil, err
	}
	x := &iNXListenToLatestMilestoneClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type INX_ListenToLatestMilestoneClient interface {
	Recv() (*Milestone, error)
	grpc.ClientStream
}

type iNXListenToLatestMilestoneClient struct {
	grpc.ClientStream
}

func (x *iNXListenToLatestMilestoneClient) Recv() (*Milestone, error) {
	m := new(Milestone)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *iNXClient) ListenToConfirmedMilestone(ctx context.Context, in *NoParams, opts ...grpc.CallOption) (INX_ListenToConfirmedMilestoneClient, error) {
	stream, err := c.cc.NewStream(ctx, &INX_ServiceDesc.Streams[1], "/inx.INX/ListenToConfirmedMilestone", opts...)
	if err != nil {
		return nil, err
	}
	x := &iNXListenToConfirmedMilestoneClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type INX_ListenToConfirmedMilestoneClient interface {
	Recv() (*Milestone, error)
	grpc.ClientStream
}

type iNXListenToConfirmedMilestoneClient struct {
	grpc.ClientStream
}

func (x *iNXListenToConfirmedMilestoneClient) Recv() (*Milestone, error) {
	m := new(Milestone)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *iNXClient) ListenToMessages(ctx context.Context, in *MessageFilter, opts ...grpc.CallOption) (INX_ListenToMessagesClient, error) {
	stream, err := c.cc.NewStream(ctx, &INX_ServiceDesc.Streams[2], "/inx.INX/ListenToMessages", opts...)
	if err != nil {
		return nil, err
	}
	x := &iNXListenToMessagesClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type INX_ListenToMessagesClient interface {
	Recv() (*Message, error)
	grpc.ClientStream
}

type iNXListenToMessagesClient struct {
	grpc.ClientStream
}

func (x *iNXListenToMessagesClient) Recv() (*Message, error) {
	m := new(Message)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *iNXClient) ListenToSolidMessages(ctx context.Context, in *MessageFilter, opts ...grpc.CallOption) (INX_ListenToSolidMessagesClient, error) {
	stream, err := c.cc.NewStream(ctx, &INX_ServiceDesc.Streams[3], "/inx.INX/ListenToSolidMessages", opts...)
	if err != nil {
		return nil, err
	}
	x := &iNXListenToSolidMessagesClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type INX_ListenToSolidMessagesClient interface {
	Recv() (*MessageMetadata, error)
	grpc.ClientStream
}

type iNXListenToSolidMessagesClient struct {
	grpc.ClientStream
}

func (x *iNXListenToSolidMessagesClient) Recv() (*MessageMetadata, error) {
	m := new(MessageMetadata)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *iNXClient) ListenToReferencedMessages(ctx context.Context, in *MessageFilter, opts ...grpc.CallOption) (INX_ListenToReferencedMessagesClient, error) {
	stream, err := c.cc.NewStream(ctx, &INX_ServiceDesc.Streams[4], "/inx.INX/ListenToReferencedMessages", opts...)
	if err != nil {
		return nil, err
	}
	x := &iNXListenToReferencedMessagesClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type INX_ListenToReferencedMessagesClient interface {
	Recv() (*MessageMetadata, error)
	grpc.ClientStream
}

type iNXListenToReferencedMessagesClient struct {
	grpc.ClientStream
}

func (x *iNXListenToReferencedMessagesClient) Recv() (*MessageMetadata, error) {
	m := new(MessageMetadata)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *iNXClient) SubmitMessage(ctx context.Context, in *RawMessage, opts ...grpc.CallOption) (*MessageId, error) {
	out := new(MessageId)
	err := c.cc.Invoke(ctx, "/inx.INX/SubmitMessage", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *iNXClient) ReadMessage(ctx context.Context, in *MessageId, opts ...grpc.CallOption) (*RawMessage, error) {
	out := new(RawMessage)
	err := c.cc.Invoke(ctx, "/inx.INX/ReadMessage", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *iNXClient) ReadMessageMetadata(ctx context.Context, in *MessageId, opts ...grpc.CallOption) (*MessageMetadata, error) {
	out := new(MessageMetadata)
	err := c.cc.Invoke(ctx, "/inx.INX/ReadMessageMetadata", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *iNXClient) ReadUnspentOutputs(ctx context.Context, in *NoParams, opts ...grpc.CallOption) (INX_ReadUnspentOutputsClient, error) {
	stream, err := c.cc.NewStream(ctx, &INX_ServiceDesc.Streams[5], "/inx.INX/ReadUnspentOutputs", opts...)
	if err != nil {
		return nil, err
	}
	x := &iNXReadUnspentOutputsClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type INX_ReadUnspentOutputsClient interface {
	Recv() (*UnspentOutput, error)
	grpc.ClientStream
}

type iNXReadUnspentOutputsClient struct {
	grpc.ClientStream
}

func (x *iNXReadUnspentOutputsClient) Recv() (*UnspentOutput, error) {
	m := new(UnspentOutput)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *iNXClient) ListenToLedgerUpdates(ctx context.Context, in *LedgerUpdateRequest, opts ...grpc.CallOption) (INX_ListenToLedgerUpdatesClient, error) {
	stream, err := c.cc.NewStream(ctx, &INX_ServiceDesc.Streams[6], "/inx.INX/ListenToLedgerUpdates", opts...)
	if err != nil {
		return nil, err
	}
	x := &iNXListenToLedgerUpdatesClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type INX_ListenToLedgerUpdatesClient interface {
	Recv() (*LedgerUpdate, error)
	grpc.ClientStream
}

type iNXListenToLedgerUpdatesClient struct {
	grpc.ClientStream
}

func (x *iNXListenToLedgerUpdatesClient) Recv() (*LedgerUpdate, error) {
	m := new(LedgerUpdate)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *iNXClient) ReadOutput(ctx context.Context, in *OutputId, opts ...grpc.CallOption) (*OutputResponse, error) {
	out := new(OutputResponse)
	err := c.cc.Invoke(ctx, "/inx.INX/ReadOutput", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *iNXClient) ListenToMigrationReceipts(ctx context.Context, in *NoParams, opts ...grpc.CallOption) (INX_ListenToMigrationReceiptsClient, error) {
	stream, err := c.cc.NewStream(ctx, &INX_ServiceDesc.Streams[7], "/inx.INX/ListenToMigrationReceipts", opts...)
	if err != nil {
		return nil, err
	}
	x := &iNXListenToMigrationReceiptsClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type INX_ListenToMigrationReceiptsClient interface {
	Recv() (*RawReceipt, error)
	grpc.ClientStream
}

type iNXListenToMigrationReceiptsClient struct {
	grpc.ClientStream
}

func (x *iNXListenToMigrationReceiptsClient) Recv() (*RawReceipt, error) {
	m := new(RawReceipt)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *iNXClient) RegisterAPIRoute(ctx context.Context, in *APIRouteRequest, opts ...grpc.CallOption) (*NoParams, error) {
	out := new(NoParams)
	err := c.cc.Invoke(ctx, "/inx.INX/RegisterAPIRoute", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *iNXClient) UnregisterAPIRoute(ctx context.Context, in *APIRouteRequest, opts ...grpc.CallOption) (*NoParams, error) {
	out := new(NoParams)
	err := c.cc.Invoke(ctx, "/inx.INX/UnregisterAPIRoute", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// INXServer is the server API for INX service.
// All implementations must embed UnimplementedINXServer
// for forward compatibility
type INXServer interface {
	// Node
	ReadNodeStatus(context.Context, *NoParams) (*NodeStatus, error)
	ReadProtocolParameters(context.Context, *NoParams) (*ProtocolParameters, error)
	// Milestones
	ReadMilestone(context.Context, *MilestoneRequest) (*Milestone, error)
	ListenToLatestMilestone(*NoParams, INX_ListenToLatestMilestoneServer) error
	ListenToConfirmedMilestone(*NoParams, INX_ListenToConfirmedMilestoneServer) error
	// Messages
	ListenToMessages(*MessageFilter, INX_ListenToMessagesServer) error
	ListenToSolidMessages(*MessageFilter, INX_ListenToSolidMessagesServer) error
	ListenToReferencedMessages(*MessageFilter, INX_ListenToReferencedMessagesServer) error
	SubmitMessage(context.Context, *RawMessage) (*MessageId, error)
	ReadMessage(context.Context, *MessageId) (*RawMessage, error)
	ReadMessageMetadata(context.Context, *MessageId) (*MessageMetadata, error)
	// UTXO
	ReadUnspentOutputs(*NoParams, INX_ReadUnspentOutputsServer) error
	ListenToLedgerUpdates(*LedgerUpdateRequest, INX_ListenToLedgerUpdatesServer) error
	ReadOutput(context.Context, *OutputId) (*OutputResponse, error)
	ListenToMigrationReceipts(*NoParams, INX_ListenToMigrationReceiptsServer) error
	// REST API
	RegisterAPIRoute(context.Context, *APIRouteRequest) (*NoParams, error)
	UnregisterAPIRoute(context.Context, *APIRouteRequest) (*NoParams, error)
	mustEmbedUnimplementedINXServer()
}

// UnimplementedINXServer must be embedded to have forward compatible implementations.
type UnimplementedINXServer struct {
}

func (UnimplementedINXServer) ReadNodeStatus(context.Context, *NoParams) (*NodeStatus, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadNodeStatus not implemented")
}
func (UnimplementedINXServer) ReadProtocolParameters(context.Context, *NoParams) (*ProtocolParameters, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadProtocolParameters not implemented")
}
func (UnimplementedINXServer) ReadMilestone(context.Context, *MilestoneRequest) (*Milestone, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadMilestone not implemented")
}
func (UnimplementedINXServer) ListenToLatestMilestone(*NoParams, INX_ListenToLatestMilestoneServer) error {
	return status.Errorf(codes.Unimplemented, "method ListenToLatestMilestone not implemented")
}
func (UnimplementedINXServer) ListenToConfirmedMilestone(*NoParams, INX_ListenToConfirmedMilestoneServer) error {
	return status.Errorf(codes.Unimplemented, "method ListenToConfirmedMilestone not implemented")
}
func (UnimplementedINXServer) ListenToMessages(*MessageFilter, INX_ListenToMessagesServer) error {
	return status.Errorf(codes.Unimplemented, "method ListenToMessages not implemented")
}
func (UnimplementedINXServer) ListenToSolidMessages(*MessageFilter, INX_ListenToSolidMessagesServer) error {
	return status.Errorf(codes.Unimplemented, "method ListenToSolidMessages not implemented")
}
func (UnimplementedINXServer) ListenToReferencedMessages(*MessageFilter, INX_ListenToReferencedMessagesServer) error {
	return status.Errorf(codes.Unimplemented, "method ListenToReferencedMessages not implemented")
}
func (UnimplementedINXServer) SubmitMessage(context.Context, *RawMessage) (*MessageId, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SubmitMessage not implemented")
}
func (UnimplementedINXServer) ReadMessage(context.Context, *MessageId) (*RawMessage, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadMessage not implemented")
}
func (UnimplementedINXServer) ReadMessageMetadata(context.Context, *MessageId) (*MessageMetadata, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadMessageMetadata not implemented")
}
func (UnimplementedINXServer) ReadUnspentOutputs(*NoParams, INX_ReadUnspentOutputsServer) error {
	return status.Errorf(codes.Unimplemented, "method ReadUnspentOutputs not implemented")
}
func (UnimplementedINXServer) ListenToLedgerUpdates(*LedgerUpdateRequest, INX_ListenToLedgerUpdatesServer) error {
	return status.Errorf(codes.Unimplemented, "method ListenToLedgerUpdates not implemented")
}
func (UnimplementedINXServer) ReadOutput(context.Context, *OutputId) (*OutputResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadOutput not implemented")
}
func (UnimplementedINXServer) ListenToMigrationReceipts(*NoParams, INX_ListenToMigrationReceiptsServer) error {
	return status.Errorf(codes.Unimplemented, "method ListenToMigrationReceipts not implemented")
}
func (UnimplementedINXServer) RegisterAPIRoute(context.Context, *APIRouteRequest) (*NoParams, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterAPIRoute not implemented")
}
func (UnimplementedINXServer) UnregisterAPIRoute(context.Context, *APIRouteRequest) (*NoParams, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UnregisterAPIRoute not implemented")
}
func (UnimplementedINXServer) mustEmbedUnimplementedINXServer() {}

// UnsafeINXServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to INXServer will
// result in compilation errors.
type UnsafeINXServer interface {
	mustEmbedUnimplementedINXServer()
}

func RegisterINXServer(s grpc.ServiceRegistrar, srv INXServer) {
	s.RegisterService(&INX_ServiceDesc, srv)
}

func _INX_ReadNodeStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NoParams)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(INXServer).ReadNodeStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/inx.INX/ReadNodeStatus",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(INXServer).ReadNodeStatus(ctx, req.(*NoParams))
	}
	return interceptor(ctx, in, info, handler)
}

func _INX_ReadProtocolParameters_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NoParams)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(INXServer).ReadProtocolParameters(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/inx.INX/ReadProtocolParameters",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(INXServer).ReadProtocolParameters(ctx, req.(*NoParams))
	}
	return interceptor(ctx, in, info, handler)
}

func _INX_ReadMilestone_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MilestoneRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(INXServer).ReadMilestone(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/inx.INX/ReadMilestone",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(INXServer).ReadMilestone(ctx, req.(*MilestoneRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _INX_ListenToLatestMilestone_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(NoParams)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(INXServer).ListenToLatestMilestone(m, &iNXListenToLatestMilestoneServer{stream})
}

type INX_ListenToLatestMilestoneServer interface {
	Send(*Milestone) error
	grpc.ServerStream
}

type iNXListenToLatestMilestoneServer struct {
	grpc.ServerStream
}

func (x *iNXListenToLatestMilestoneServer) Send(m *Milestone) error {
	return x.ServerStream.SendMsg(m)
}

func _INX_ListenToConfirmedMilestone_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(NoParams)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(INXServer).ListenToConfirmedMilestone(m, &iNXListenToConfirmedMilestoneServer{stream})
}

type INX_ListenToConfirmedMilestoneServer interface {
	Send(*Milestone) error
	grpc.ServerStream
}

type iNXListenToConfirmedMilestoneServer struct {
	grpc.ServerStream
}

func (x *iNXListenToConfirmedMilestoneServer) Send(m *Milestone) error {
	return x.ServerStream.SendMsg(m)
}

func _INX_ListenToMessages_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(MessageFilter)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(INXServer).ListenToMessages(m, &iNXListenToMessagesServer{stream})
}

type INX_ListenToMessagesServer interface {
	Send(*Message) error
	grpc.ServerStream
}

type iNXListenToMessagesServer struct {
	grpc.ServerStream
}

func (x *iNXListenToMessagesServer) Send(m *Message) error {
	return x.ServerStream.SendMsg(m)
}

func _INX_ListenToSolidMessages_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(MessageFilter)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(INXServer).ListenToSolidMessages(m, &iNXListenToSolidMessagesServer{stream})
}

type INX_ListenToSolidMessagesServer interface {
	Send(*MessageMetadata) error
	grpc.ServerStream
}

type iNXListenToSolidMessagesServer struct {
	grpc.ServerStream
}

func (x *iNXListenToSolidMessagesServer) Send(m *MessageMetadata) error {
	return x.ServerStream.SendMsg(m)
}

func _INX_ListenToReferencedMessages_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(MessageFilter)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(INXServer).ListenToReferencedMessages(m, &iNXListenToReferencedMessagesServer{stream})
}

type INX_ListenToReferencedMessagesServer interface {
	Send(*MessageMetadata) error
	grpc.ServerStream
}

type iNXListenToReferencedMessagesServer struct {
	grpc.ServerStream
}

func (x *iNXListenToReferencedMessagesServer) Send(m *MessageMetadata) error {
	return x.ServerStream.SendMsg(m)
}

func _INX_SubmitMessage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RawMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(INXServer).SubmitMessage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/inx.INX/SubmitMessage",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(INXServer).SubmitMessage(ctx, req.(*RawMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _INX_ReadMessage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MessageId)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(INXServer).ReadMessage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/inx.INX/ReadMessage",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(INXServer).ReadMessage(ctx, req.(*MessageId))
	}
	return interceptor(ctx, in, info, handler)
}

func _INX_ReadMessageMetadata_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MessageId)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(INXServer).ReadMessageMetadata(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/inx.INX/ReadMessageMetadata",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(INXServer).ReadMessageMetadata(ctx, req.(*MessageId))
	}
	return interceptor(ctx, in, info, handler)
}

func _INX_ReadUnspentOutputs_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(NoParams)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(INXServer).ReadUnspentOutputs(m, &iNXReadUnspentOutputsServer{stream})
}

type INX_ReadUnspentOutputsServer interface {
	Send(*UnspentOutput) error
	grpc.ServerStream
}

type iNXReadUnspentOutputsServer struct {
	grpc.ServerStream
}

func (x *iNXReadUnspentOutputsServer) Send(m *UnspentOutput) error {
	return x.ServerStream.SendMsg(m)
}

func _INX_ListenToLedgerUpdates_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(LedgerUpdateRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(INXServer).ListenToLedgerUpdates(m, &iNXListenToLedgerUpdatesServer{stream})
}

type INX_ListenToLedgerUpdatesServer interface {
	Send(*LedgerUpdate) error
	grpc.ServerStream
}

type iNXListenToLedgerUpdatesServer struct {
	grpc.ServerStream
}

func (x *iNXListenToLedgerUpdatesServer) Send(m *LedgerUpdate) error {
	return x.ServerStream.SendMsg(m)
}

func _INX_ReadOutput_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(OutputId)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(INXServer).ReadOutput(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/inx.INX/ReadOutput",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(INXServer).ReadOutput(ctx, req.(*OutputId))
	}
	return interceptor(ctx, in, info, handler)
}

func _INX_ListenToMigrationReceipts_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(NoParams)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(INXServer).ListenToMigrationReceipts(m, &iNXListenToMigrationReceiptsServer{stream})
}

type INX_ListenToMigrationReceiptsServer interface {
	Send(*RawReceipt) error
	grpc.ServerStream
}

type iNXListenToMigrationReceiptsServer struct {
	grpc.ServerStream
}

func (x *iNXListenToMigrationReceiptsServer) Send(m *RawReceipt) error {
	return x.ServerStream.SendMsg(m)
}

func _INX_RegisterAPIRoute_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(APIRouteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(INXServer).RegisterAPIRoute(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/inx.INX/RegisterAPIRoute",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(INXServer).RegisterAPIRoute(ctx, req.(*APIRouteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _INX_UnregisterAPIRoute_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(APIRouteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(INXServer).UnregisterAPIRoute(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/inx.INX/UnregisterAPIRoute",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(INXServer).UnregisterAPIRoute(ctx, req.(*APIRouteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// INX_ServiceDesc is the grpc.ServiceDesc for INX service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var INX_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "inx.INX",
	HandlerType: (*INXServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ReadNodeStatus",
			Handler:    _INX_ReadNodeStatus_Handler,
		},
		{
			MethodName: "ReadProtocolParameters",
			Handler:    _INX_ReadProtocolParameters_Handler,
		},
		{
			MethodName: "ReadMilestone",
			Handler:    _INX_ReadMilestone_Handler,
		},
		{
			MethodName: "SubmitMessage",
			Handler:    _INX_SubmitMessage_Handler,
		},
		{
			MethodName: "ReadMessage",
			Handler:    _INX_ReadMessage_Handler,
		},
		{
			MethodName: "ReadMessageMetadata",
			Handler:    _INX_ReadMessageMetadata_Handler,
		},
		{
			MethodName: "ReadOutput",
			Handler:    _INX_ReadOutput_Handler,
		},
		{
			MethodName: "RegisterAPIRoute",
			Handler:    _INX_RegisterAPIRoute_Handler,
		},
		{
			MethodName: "UnregisterAPIRoute",
			Handler:    _INX_UnregisterAPIRoute_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ListenToLatestMilestone",
			Handler:       _INX_ListenToLatestMilestone_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "ListenToConfirmedMilestone",
			Handler:       _INX_ListenToConfirmedMilestone_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "ListenToMessages",
			Handler:       _INX_ListenToMessages_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "ListenToSolidMessages",
			Handler:       _INX_ListenToSolidMessages_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "ListenToReferencedMessages",
			Handler:       _INX_ListenToReferencedMessages_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "ReadUnspentOutputs",
			Handler:       _INX_ReadUnspentOutputs_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "ListenToLedgerUpdates",
			Handler:       _INX_ListenToLedgerUpdates_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "ListenToMigrationReceipts",
			Handler:       _INX_ListenToMigrationReceipts_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "inx.proto",
}
