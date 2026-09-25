// Copyright 2026 Synadia Communications Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package migrate

// tableVersion is the nats.go version every entry was verified against.
const tableVersion = "v1.53.1"

// Kind says how a legacy symbol migrates.
type Kind int

const (
	// Rename: a same-shaped jetstream counterpart (types, enum constants,
	// error values, functions).
	Rename Kind = iota + 1
	// Same: a method the receiver's jetstream counterpart has unchanged; it
	// migrates only through its receiver's type.
	Same
	// Call: a method call rewritten to a jetstream call, as Shape says.
	Call
	// Option: an option function mapped to a jetstream option function.
	Option
	// Field: a subscribe option folded into consumer config fields.
	Field
	// Subscribe: a legacy subscribe call; its site raises decisions.
	Subscribe
	// Guided: a determined transformation whose text depends on the
	// surrounding code; Note carries the facts.
	Guided
	// BySite: a symbol handled by the site that uses it (option types,
	// options that select a call shape); it has no rewrite of its own.
	BySite
	// Unmapped: no jetstream equivalent; Note says why.
	Unmapped
)

// Shape flags how a Call's arguments change.
type Shape int

const (
	// AddCtx inserts the context argument first.
	AddCtx Shape = 1 << iota
	// PtrToValue passes the first config argument by value.
	PtrToValue
	// ViaStream calls Target on js.Stream(ctx, <first argument>).
	ViaStream
	// ViaConsumer calls Target on js.Consumer(ctx, <first>, <second>).
	ViaConsumer
	// Constructor replaces the call with a jetstream constructor taking the
	// receiver as its first argument.
	Constructor
)

// FieldValue is one consumer config field a subscribe option sets; in
// Value, "$1" is the option's first argument and "$*" all its arguments.
type FieldValue struct {
	Name  string
	Value string
}

// Entry says what one legacy symbol becomes.
type Entry struct {
	Kind Kind
	// Target is the jetstream counterpart, "Name" or "Type.Member".
	Target string
	Shape  Shape
	Fields []FieldValue
	// Accessor is the lister method whose channel stands in for the legacy
	// channel result.
	Accessor string
	// Ref is the MIGRATION.md section the entry corresponds to.
	Ref string
	// Note is the reason (Unmapped), the facts (Guided), the handling
	// (BySite) or a note shown on every site.
	Note string
}

// Sections of nats.go's jetstream/MIGRATION.md.
const (
	refInit      = "#initialization-options"
	refStreams   = "#stream-management"
	refConsumers = "#consumer-management"
	refPublish   = "#publishing"
	refPubOpts   = "#publish-options"
	refSubscribe = "#replacing-jssubscribe"
	refPull      = "#replacing-jspullsubscribe"
	refOrdered   = "#ordered-consumers"
	refPush      = "#push-consumers"
	refSubOpts   = "#subscription-options-mapping"
	refAck       = "#message-acknowledgement"
	refKV        = "#keyvalue-store"
	refKVMgmt    = "#kv-management-methods"
	refObj       = "#object-store"
	refObjMgmt   = "#object-store-management-methods"
)

const (
	noteStreamInfo  = "the new call first obtains a stream handle, which sends a STREAM.INFO request and needs that permission"
	noteListerErr   = "legacy swallowed listing errors; ranging over the lister's channel keeps that, check the lister's error as a follow-up"
	notePushOnly    = "push consumers only; on a pull consumer it is a push-only-option decision"
	noteContextOnly = "carries only a context, which becomes the new call's ctx argument"
	noteMarshal     = "JSON marshaling, unchanged on the jetstream type"
)

var table = map[string]Entry{
	// Handle and handle options.
	"Conn.JetStream":            {Kind: Call, Target: "New", Shape: Constructor, Ref: refInit},
	"JetStreamContext":          {Kind: Rename, Target: "JetStream", Ref: refInit},
	"JetStream":                 {Kind: Rename, Target: "JetStream", Ref: refInit},
	"JetStreamManager":          {Kind: Rename, Target: "JetStream", Ref: refInit},
	"JSOpt":                     {Kind: BySite, Target: "JetStreamOpt", Note: "handle options map one by one to jetstream.JetStreamOpt or to a constructor"},
	"APIPrefix":                 {Kind: Option, Target: "NewWithAPIPrefix", Ref: refInit, Note: "selects the constructor at the handle root"},
	"Domain":                    {Kind: Option, Target: "NewWithDomain", Ref: refInit, Note: "selects the constructor at the handle root"},
	"ClientTrace":               {Kind: Option, Target: "WithClientTrace", Ref: refInit},
	"PublishAsyncErrHandler":    {Kind: Option, Target: "WithPublishAsyncErrHandler", Ref: refInit},
	"PublishAsyncMaxPending":    {Kind: Option, Target: "WithPublishAsyncMaxPending", Ref: refInit},
	"PublishAsyncTimeout":       {Kind: Option, Target: "WithPublishAsyncTimeout", Ref: refInit},
	"MaxWait":                   {Kind: Option, Target: "WithDefaultTimeout", Ref: refInit, Note: "a handle option becomes WithDefaultTimeout; on a single call it is a guided context.WithTimeout, on Fetch it is FetchMaxWait"},
	"UseLegacyDurableConsumers": {Kind: Unmapped, Note: "the jetstream package always uses the consumer create API; there is no legacy mode"},
	"DirectGet":                 {Kind: Unmapped, Note: "jetstream has no direct-get switch on the handle; stream.GetMsg decides the API"},
	"DirectGetNext":             {Kind: Unmapped, Note: "jetstream has no direct-get-next option; use stream.GetMsg or GetLastMsgForSubject"},
	"Context":                   {Kind: BySite, Note: noteContextOnly},
	"ContextOpt":                {Kind: BySite, Note: noteContextOnly},
	"MsgErrHandler":             {Kind: Rename, Target: "MsgErrHandler", Ref: refInit, Note: "the callback's first parameter becomes jetstream.JetStream"},

	// Account and shared types.
	"APIError":                     {Kind: Rename, Target: "APIError"},
	"APIError.APIError":            {Kind: Same, Target: "APIError.APIError"},
	"APIError.Error":               {Kind: Same, Target: "APIError.Error"},
	"APIError.Is":                  {Kind: Same, Target: "APIError.Is"},
	"JetStreamError":               {Kind: Rename, Target: "JetStreamError"},
	"JetStreamError.APIError":      {Kind: Same, Target: "JetStreamError.APIError"},
	"ErrorCode":                    {Kind: Rename, Target: "ErrorCode"},
	"APIStats":                     {Kind: Rename, Target: "APIStats"},
	"AccountInfo":                  {Kind: Rename, Target: "AccountInfo"},
	"AccountLimits":                {Kind: Rename, Target: "AccountLimits"},
	"Tier":                         {Kind: Rename, Target: "Tier"},
	"ClusterInfo":                  {Kind: Rename, Target: "ClusterInfo"},
	"PeerInfo":                     {Kind: Rename, Target: "PeerInfo"},
	"Placement":                    {Kind: Rename, Target: "Placement"},
	"JetStreamManager.AccountInfo": {Kind: Call, Target: "JetStream.AccountInfo", Shape: AddCtx},

	// Streams.
	"StreamConfig":                         {Kind: Rename, Target: "StreamConfig", Ref: refStreams},
	"StreamInfo":                           {Kind: Rename, Target: "StreamInfo", Ref: refStreams},
	"StreamState":                          {Kind: Rename, Target: "StreamState", Ref: refStreams},
	"StreamSource":                         {Kind: Rename, Target: "StreamSource", Ref: refStreams},
	"StreamSourceInfo":                     {Kind: Rename, Target: "StreamSourceInfo", Ref: refStreams},
	"StreamConsumerLimits":                 {Kind: Rename, Target: "StreamConsumerLimits", Ref: refStreams},
	"StreamAlternate":                      {Kind: Unmapped, Note: "jetstream.StreamInfo has no alternates"},
	"ExternalStream":                       {Kind: Rename, Target: "ExternalStream", Ref: refStreams},
	"SubjectTransformConfig":               {Kind: Rename, Target: "SubjectTransformConfig", Ref: refStreams},
	"RePublish":                            {Kind: Rename, Target: "RePublish", Ref: refStreams},
	"RawStreamMsg":                         {Kind: Rename, Target: "RawStreamMsg", Ref: refStreams},
	"StreamPurgeRequest":                   {Kind: Guided, Target: "WithPurgeSubject", Ref: refStreams, Note: "a purge request becomes StreamPurgeOpt options: WithPurgeSubject, WithPurgeSequence, WithPurgeKeep"},
	"StreamInfoRequest":                    {Kind: Guided, Target: "WithSubjectFilter", Ref: refStreams, Note: "a stream info request becomes StreamInfoOpt options: WithSubjectFilter, WithDeletedDetails"},
	"StreamListFilter":                     {Kind: Option, Target: "WithStreamListSubject", Ref: refStreams},
	"JetStreamManager.AddStream":           {Kind: Call, Target: "JetStream.CreateStream", Shape: AddCtx | PtrToValue, Ref: refStreams},
	"JetStreamManager.UpdateStream":        {Kind: Call, Target: "JetStream.UpdateStream", Shape: AddCtx | PtrToValue, Ref: refStreams},
	"JetStreamManager.DeleteStream":        {Kind: Call, Target: "JetStream.DeleteStream", Shape: AddCtx, Ref: refStreams},
	"JetStreamManager.StreamNameBySubject": {Kind: Call, Target: "JetStream.StreamNameBySubject", Shape: AddCtx, Ref: refStreams},
	"JetStreamManager.StreamInfo":          {Kind: Call, Target: "Stream.Info", Shape: ViaStream, Ref: refStreams, Note: noteStreamInfo},
	"JetStreamManager.PurgeStream":         {Kind: Call, Target: "Stream.Purge", Shape: ViaStream, Ref: refStreams, Note: noteStreamInfo},
	"JetStreamManager.GetMsg":              {Kind: Call, Target: "Stream.GetMsg", Shape: ViaStream, Ref: refStreams, Note: noteStreamInfo},
	"JetStreamManager.GetLastMsg":          {Kind: Call, Target: "Stream.GetLastMsgForSubject", Shape: ViaStream, Ref: refStreams, Note: noteStreamInfo},
	"JetStreamManager.DeleteMsg":           {Kind: Call, Target: "Stream.DeleteMsg", Shape: ViaStream, Ref: refStreams, Note: noteStreamInfo},
	"JetStreamManager.SecureDeleteMsg":     {Kind: Call, Target: "Stream.SecureDeleteMsg", Shape: ViaStream, Ref: refStreams, Note: noteStreamInfo},
	"JetStreamManager.Streams":             {Kind: Call, Target: "JetStream.ListStreams", Shape: AddCtx, Accessor: "Info", Ref: refStreams, Note: noteListerErr},
	"JetStreamManager.StreamsInfo":         {Kind: Call, Target: "JetStream.ListStreams", Shape: AddCtx, Accessor: "Info", Ref: refStreams, Note: noteListerErr},
	"JetStreamManager.StreamNames":         {Kind: Call, Target: "JetStream.StreamNames", Shape: AddCtx, Accessor: "Name", Ref: refStreams, Note: noteListerErr},

	// Enums.
	"AckPolicy":                      {Kind: Rename, Target: "AckPolicy"},
	"AckPolicy.MarshalJSON":          {Kind: Same, Target: "AckPolicy.MarshalJSON", Note: noteMarshal},
	"AckPolicy.UnmarshalJSON":        {Kind: Same, Target: "AckPolicy.UnmarshalJSON", Note: noteMarshal},
	"AckPolicy.String":               {Kind: Same, Target: "AckPolicy.String"},
	"DeliverPolicy":                  {Kind: Rename, Target: "DeliverPolicy"},
	"DeliverPolicy.MarshalJSON":      {Kind: Same, Target: "DeliverPolicy.MarshalJSON", Note: noteMarshal},
	"DeliverPolicy.UnmarshalJSON":    {Kind: Same, Target: "DeliverPolicy.UnmarshalJSON", Note: noteMarshal},
	"DiscardPolicy":                  {Kind: Rename, Target: "DiscardPolicy"},
	"DiscardPolicy.MarshalJSON":      {Kind: Same, Target: "DiscardPolicy.MarshalJSON", Note: noteMarshal},
	"DiscardPolicy.UnmarshalJSON":    {Kind: Same, Target: "DiscardPolicy.UnmarshalJSON", Note: noteMarshal},
	"DiscardPolicy.String":           {Kind: Same, Target: "DiscardPolicy.String"},
	"ReplayPolicy":                   {Kind: Rename, Target: "ReplayPolicy"},
	"ReplayPolicy.MarshalJSON":       {Kind: Same, Target: "ReplayPolicy.MarshalJSON", Note: noteMarshal},
	"ReplayPolicy.UnmarshalJSON":     {Kind: Same, Target: "ReplayPolicy.UnmarshalJSON", Note: noteMarshal},
	"RetentionPolicy":                {Kind: Rename, Target: "RetentionPolicy"},
	"RetentionPolicy.MarshalJSON":    {Kind: Same, Target: "RetentionPolicy.MarshalJSON", Note: noteMarshal},
	"RetentionPolicy.UnmarshalJSON":  {Kind: Same, Target: "RetentionPolicy.UnmarshalJSON", Note: noteMarshal},
	"RetentionPolicy.String":         {Kind: Same, Target: "RetentionPolicy.String"},
	"StorageType":                    {Kind: Rename, Target: "StorageType"},
	"StorageType.MarshalJSON":        {Kind: Same, Target: "StorageType.MarshalJSON", Note: noteMarshal},
	"StorageType.UnmarshalJSON":      {Kind: Same, Target: "StorageType.UnmarshalJSON", Note: noteMarshal},
	"StorageType.String":             {Kind: Same, Target: "StorageType.String"},
	"StoreCompression":               {Kind: Rename, Target: "StoreCompression"},
	"StoreCompression.MarshalJSON":   {Kind: Same, Target: "StoreCompression.MarshalJSON", Note: noteMarshal},
	"StoreCompression.UnmarshalJSON": {Kind: Same, Target: "StoreCompression.UnmarshalJSON", Note: noteMarshal},
	"StoreCompression.String":        {Kind: Same, Target: "StoreCompression.String"},

	// Consumers.
	"ConsumerConfig":                  {Kind: Rename, Target: "ConsumerConfig", Ref: refConsumers, Note: "field Heartbeat is IdleHeartbeat"},
	"ConsumerInfo":                    {Kind: Rename, Target: "ConsumerInfo", Ref: refConsumers},
	"SequenceInfo":                    {Kind: Rename, Target: "SequenceInfo", Ref: refConsumers},
	"SequencePair":                    {Kind: Rename, Target: "SequencePair", Ref: refConsumers},
	"JetStreamManager.AddConsumer":    {Kind: Call, Target: "JetStream.CreateConsumer", Shape: AddCtx | PtrToValue, Ref: refConsumers, Note: "legacy AddConsumer is create-only: it returns an identical existing consumer and fails with ErrConsumerNameAlreadyInUse otherwise"},
	"JetStreamManager.UpdateConsumer": {Kind: Call, Target: "JetStream.UpdateConsumer", Shape: AddCtx | PtrToValue, Ref: refConsumers},
	"JetStreamManager.DeleteConsumer": {Kind: Call, Target: "JetStream.DeleteConsumer", Shape: AddCtx, Ref: refConsumers},
	"JetStreamManager.ConsumerInfo":   {Kind: Call, Target: "Consumer.Info", Shape: ViaConsumer, Ref: refConsumers, Note: "the new call first obtains a consumer handle, which sends a CONSUMER.INFO request"},
	"JetStreamManager.ConsumerNames":  {Kind: Guided, Target: "Stream.ConsumerNames", Ref: refConsumers, Note: "consumer listers live on jetstream.Stream: obtain js.Stream(ctx, name) with its own error handling, then range over ConsumerNames(ctx).Name()"},
	"JetStreamManager.Consumers":      {Kind: Guided, Target: "Stream.ListConsumers", Ref: refConsumers, Note: "consumer listers live on jetstream.Stream: obtain js.Stream(ctx, name) with its own error handling, then range over ListConsumers(ctx).Info()"},
	"JetStreamManager.ConsumersInfo":  {Kind: Guided, Target: "Stream.ListConsumers", Ref: refConsumers, Note: "consumer listers live on jetstream.Stream: obtain js.Stream(ctx, name) with its own error handling, then range over ListConsumers(ctx).Info()"},

	// Subscriptions.
	"JetStream.Subscribe":               {Kind: Subscribe, Ref: refSubscribe},
	"JetStream.SubscribeSync":           {Kind: Subscribe, Ref: refSubscribe},
	"JetStream.QueueSubscribe":          {Kind: Subscribe, Ref: refSubscribe},
	"JetStream.QueueSubscribeSync":      {Kind: Subscribe, Ref: refSubscribe},
	"JetStream.ChanSubscribe":           {Kind: Subscribe, Ref: refSubscribe},
	"JetStream.ChanQueueSubscribe":      {Kind: Subscribe, Ref: refSubscribe},
	"JetStream.PullSubscribe":           {Kind: Subscribe, Ref: refPull},
	"SubOpt":                            {Kind: BySite, Note: "subscribe options become consumer config fields or select the consumer call"},
	"Bind":                              {Kind: BySite, Target: "JetStream.Consumer", Ref: refSubOpts, Note: "a bound subscription uses the existing consumer: js.Consumer(ctx, stream, consumer)"},
	"BindStream":                        {Kind: BySite, Target: "JetStream.Stream", Ref: refSubOpts, Note: "names the stream; no runtime stream lookup is needed"},
	"OrderedConsumer":                   {Kind: BySite, Target: "JetStream.OrderedConsumer", Ref: refOrdered, Note: "an ordered subscription becomes js.OrderedConsumer(ctx, stream, OrderedConsumerConfig); it is pull-based and never acks"},
	"ManualAck":                         {Kind: BySite, Ref: refSubOpts, Note: "the jetstream package never acks for the handler; without ManualAck the site has an ack decision"},
	"SkipConsumerLookup":                {Kind: BySite, Ref: refSubOpts, Note: "creating a consumer in the jetstream package does not look it up first"},
	"Durable":                           {Kind: Field, Fields: []FieldValue{{"Durable", "$1"}}, Ref: refSubOpts},
	"ConsumerName":                      {Kind: Field, Fields: []FieldValue{{"Name", "$1"}}, Ref: refSubOpts},
	"Description":                       {Kind: Field, Fields: []FieldValue{{"Description", "$1"}}, Ref: refSubOpts},
	"DeliverAll":                        {Kind: Field, Fields: []FieldValue{{"DeliverPolicy", "jetstream.DeliverAllPolicy"}}, Ref: refSubOpts},
	"DeliverLast":                       {Kind: Field, Fields: []FieldValue{{"DeliverPolicy", "jetstream.DeliverLastPolicy"}}, Ref: refSubOpts},
	"DeliverLastPerSubject":             {Kind: Field, Fields: []FieldValue{{"DeliverPolicy", "jetstream.DeliverLastPerSubjectPolicy"}}, Ref: refSubOpts},
	"DeliverNew":                        {Kind: Field, Fields: []FieldValue{{"DeliverPolicy", "jetstream.DeliverNewPolicy"}}, Ref: refSubOpts},
	"StartSequence":                     {Kind: Field, Fields: []FieldValue{{"DeliverPolicy", "jetstream.DeliverByStartSequencePolicy"}, {"OptStartSeq", "$1"}}, Ref: refSubOpts},
	"StartTime":                         {Kind: Field, Fields: []FieldValue{{"DeliverPolicy", "jetstream.DeliverByStartTimePolicy"}, {"OptStartTime", "&$1"}}, Ref: refSubOpts},
	"AckExplicit":                       {Kind: Field, Fields: []FieldValue{{"AckPolicy", "jetstream.AckExplicitPolicy"}}, Ref: refSubOpts},
	"AckAll":                            {Kind: Field, Fields: []FieldValue{{"AckPolicy", "jetstream.AckAllPolicy"}}, Ref: refSubOpts},
	"AckNone":                           {Kind: Field, Fields: []FieldValue{{"AckPolicy", "jetstream.AckNonePolicy"}}, Ref: refSubOpts},
	"AckWait":                           {Kind: Field, Fields: []FieldValue{{"AckWait", "$1"}}, Ref: refSubOpts, Note: "as a publish option it is a per-call timeout: guided"},
	"MaxDeliver":                        {Kind: Field, Fields: []FieldValue{{"MaxDeliver", "$1"}}, Ref: refSubOpts},
	"MaxAckPending":                     {Kind: Field, Fields: []FieldValue{{"MaxAckPending", "$1"}}, Ref: refSubOpts},
	"BackOff":                           {Kind: Field, Fields: []FieldValue{{"BackOff", "$1"}}, Ref: refSubOpts},
	"ReplayOriginal":                    {Kind: Field, Fields: []FieldValue{{"ReplayPolicy", "jetstream.ReplayOriginalPolicy"}}, Ref: refSubOpts},
	"ReplayInstant":                     {Kind: Field, Fields: []FieldValue{{"ReplayPolicy", "jetstream.ReplayInstantPolicy"}}, Ref: refSubOpts},
	"RateLimit":                         {Kind: Field, Fields: []FieldValue{{"RateLimit", "$1"}}, Ref: refSubOpts, Note: notePushOnly},
	"DeliverSubject":                    {Kind: Field, Fields: []FieldValue{{"DeliverSubject", "$1"}}, Ref: refPush, Note: notePushOnly},
	"EnableFlowControl":                 {Kind: Field, Fields: []FieldValue{{"FlowControl", "true"}}, Ref: refPush, Note: notePushOnly},
	"IdleHeartbeat":                     {Kind: Field, Fields: []FieldValue{{"IdleHeartbeat", "$1"}}, Ref: refPush, Note: notePushOnly},
	"HeadersOnly":                       {Kind: Field, Fields: []FieldValue{{"HeadersOnly", "true"}}, Ref: refSubOpts},
	"InactiveThreshold":                 {Kind: Field, Fields: []FieldValue{{"InactiveThreshold", "$1"}}, Ref: refSubOpts},
	"ConsumerFilterSubjects":            {Kind: Field, Fields: []FieldValue{{"FilterSubjects", "[]string{$*}"}}, Ref: refSubOpts},
	"ConsumerReplicas":                  {Kind: Field, Fields: []FieldValue{{"Replicas", "$1"}}, Ref: refSubOpts},
	"ConsumerMemoryStorage":             {Kind: Field, Fields: []FieldValue{{"MemoryStorage", "true"}}, Ref: refSubOpts},
	"PullMaxWaiting":                    {Kind: Field, Fields: []FieldValue{{"MaxWaiting", "$1"}}, Ref: refPull},
	"MaxRequestBatch":                   {Kind: Field, Fields: []FieldValue{{"MaxRequestBatch", "$1"}}, Ref: refPull},
	"MaxRequestExpires":                 {Kind: Field, Fields: []FieldValue{{"MaxRequestExpires", "$1"}}, Ref: refPull},
	"MaxRequestMaxBytes":                {Kind: Field, Fields: []FieldValue{{"MaxRequestMaxBytes", "$1"}}, Ref: refPull},
	"ErrConsumerSequenceMismatch":       {Kind: Unmapped, Note: "reported by legacy ordered push subscriptions; the jetstream ordered consumer resets itself"},
	"ErrConsumerSequenceMismatch.Error": {Kind: Unmapped, Note: "reported by legacy ordered push subscriptions; the jetstream ordered consumer resets itself"},

	// Pulling from a subscription.
	"PullOpt":                             {Kind: BySite, Target: "FetchOpt", Ref: refPull, Note: "fetch options map to jetstream FetchOpt"},
	"PullHeartbeat":                       {Kind: Option, Target: "FetchHeartbeat", Ref: refPull},
	"PullMaxBytes":                        {Kind: Guided, Target: "Consumer.FetchBytes", Ref: refPull, Note: "a byte-limited fetch is Consumer.FetchBytes(maxBytes, ...)"},
	"Subscription.Fetch":                  {Kind: Guided, Target: "Consumer.Fetch", Ref: refPull, Note: "Fetch returns a MessageBatch: range over Messages(), then check Error(); an empty pull is not an error (timeouts and no-message statuses are dropped), so legacy nats.ErrTimeout checks on the batch go away; Consumer.Next still returns nats.ErrTimeout"},
	"Subscription.FetchBatch":             {Kind: Guided, Target: "Consumer.Fetch", Ref: refPull, Note: "FetchBatch becomes Consumer.Fetch, whose MessageBatch has the same Messages() and Error()"},
	"Subscription.ConsumerInfo":           {Kind: Guided, Target: "Consumer.Info", Ref: refConsumers, Note: "call Info(ctx), or CachedInfo(), on the consumer handle the subscription site creates"},
	"Subscription.InitialConsumerPending": {Kind: Guided, Target: "Consumer.CachedInfo", Ref: refConsumers, Note: "read NumPending from the consumer handle's CachedInfo() taken when it was created"},
	"MessageBatch":                        {Kind: Rename, Target: "MessageBatch", Ref: refPull},
	"MessageBatch.Messages":               {Kind: Same, Target: "MessageBatch.Messages", Ref: refPull},
	"MessageBatch.Error":                  {Kind: Same, Target: "MessageBatch.Error", Ref: refPull},
	"MessageBatch.Done":                   {Kind: Unmapped, Note: "jetstream.MessageBatch has no Done channel; Messages() closes when the batch ends"},

	// Messages.
	"Msg.Ack":          {Kind: Same, Target: "Msg.Ack", Ref: refAck},
	"Msg.AckSync":      {Kind: Call, Target: "Msg.DoubleAck", Shape: AddCtx, Ref: refAck},
	"Msg.Nak":          {Kind: Same, Target: "Msg.Nak", Ref: refAck},
	"Msg.NakWithDelay": {Kind: Same, Target: "Msg.NakWithDelay", Ref: refAck},
	"Msg.InProgress":   {Kind: Same, Target: "Msg.InProgress", Ref: refAck},
	"Msg.Term":         {Kind: Same, Target: "Msg.Term", Ref: refAck},
	"Msg.Metadata":     {Kind: Same, Target: "Msg.Metadata", Ref: refAck},
	"MsgMetadata":      {Kind: Rename, Target: "MsgMetadata", Ref: refAck},
	"AckOpt":           {Kind: BySite, Note: noteContextOnly},

	// Publishing.
	"JetStream.Publish":              {Kind: Call, Target: "JetStream.Publish", Shape: AddCtx, Ref: refPublish},
	"JetStream.PublishMsg":           {Kind: Call, Target: "JetStream.PublishMsg", Shape: AddCtx, Ref: refPublish},
	"JetStream.PublishAsync":         {Kind: Call, Target: "JetStream.PublishAsync", Ref: refPublish},
	"JetStream.PublishMsgAsync":      {Kind: Call, Target: "JetStream.PublishMsgAsync", Ref: refPublish},
	"JetStream.PublishAsyncPending":  {Kind: Call, Target: "JetStream.PublishAsyncPending", Ref: refPublish},
	"JetStream.PublishAsyncComplete": {Kind: Call, Target: "JetStream.PublishAsyncComplete", Ref: refPublish},
	"JetStream.CleanupPublisher":     {Kind: Call, Target: "JetStream.CleanupPublisher", Ref: refPublish},
	"PubOpt":                         {Kind: BySite, Target: "PublishOpt", Ref: refPubOpts, Note: "publish options map one by one to jetstream.PublishOpt"},
	"PubAck":                         {Kind: Rename, Target: "PubAck", Ref: refPublish},
	"PubAckFuture":                   {Kind: Rename, Target: "PubAckFuture", Ref: refPublish},
	"PubAckFuture.Ok":                {Kind: Same, Target: "PubAckFuture.Ok", Ref: refPublish},
	"PubAckFuture.Err":               {Kind: Same, Target: "PubAckFuture.Err", Ref: refPublish},
	"PubAckFuture.Msg":               {Kind: Same, Target: "PubAckFuture.Msg", Ref: refPublish},
	"MsgId":                          {Kind: Option, Target: "WithMsgID", Ref: refPubOpts},
	"ExpectStream":                   {Kind: Option, Target: "WithExpectStream", Ref: refPubOpts},
	"ExpectLastSequence":             {Kind: Option, Target: "WithExpectLastSequence", Ref: refPubOpts},
	"ExpectLastSequencePerSubject":   {Kind: Option, Target: "WithExpectLastSequencePerSubject", Ref: refPubOpts},
	"ExpectLastMsgId":                {Kind: Option, Target: "WithExpectLastMsgID", Ref: refPubOpts},
	"RetryWait":                      {Kind: Option, Target: "WithRetryWait", Ref: refPubOpts},
	"RetryAttempts":                  {Kind: Option, Target: "WithRetryAttempts", Ref: refPubOpts},
	"StallWait":                      {Kind: Option, Target: "WithStallWait", Ref: refPubOpts},
	"MsgTTL":                         {Kind: Option, Target: "WithMsgTTL", Ref: refPubOpts},

	// KeyValue.
	"KeyValueManager":                    {Kind: Rename, Target: "KeyValueManager", Ref: refKVMgmt},
	"KeyValueManager.KeyValue":           {Kind: Call, Target: "JetStream.KeyValue", Shape: AddCtx, Ref: refKVMgmt},
	"KeyValueManager.CreateKeyValue":     {Kind: Call, Target: "JetStream.CreateKeyValue", Shape: AddCtx | PtrToValue, Ref: refKVMgmt},
	"KeyValueManager.DeleteKeyValue":     {Kind: Call, Target: "JetStream.DeleteKeyValue", Shape: AddCtx, Ref: refKVMgmt},
	"KeyValueManager.KeyValueStoreNames": {Kind: Call, Target: "JetStream.KeyValueStoreNames", Shape: AddCtx, Accessor: "Name", Ref: refKVMgmt, Note: noteListerErr},
	"KeyValueManager.KeyValueStores":     {Kind: Call, Target: "JetStream.KeyValueStores", Shape: AddCtx, Accessor: "Status", Ref: refKVMgmt, Note: noteListerErr},
	"KeyValueConfig":                     {Kind: Rename, Target: "KeyValueConfig", Ref: refKV},
	"KeyValue":                           {Kind: Rename, Target: "KeyValue", Ref: refKV},
	"KeyValue.Bucket":                    {Kind: Same, Target: "KeyValue.Bucket", Ref: refKV},
	"KeyValue.Get":                       {Kind: Call, Target: "KeyValue.Get", Shape: AddCtx, Ref: refKV},
	"KeyValue.GetRevision":               {Kind: Call, Target: "KeyValue.GetRevision", Shape: AddCtx, Ref: refKV},
	"KeyValue.Put":                       {Kind: Call, Target: "KeyValue.Put", Shape: AddCtx, Ref: refKV},
	"KeyValue.PutString":                 {Kind: Call, Target: "KeyValue.PutString", Shape: AddCtx, Ref: refKV},
	"KeyValue.Create":                    {Kind: Call, Target: "KeyValue.Create", Shape: AddCtx, Ref: refKV},
	"KeyValue.Update":                    {Kind: Call, Target: "KeyValue.Update", Shape: AddCtx, Ref: refKV},
	"KeyValue.Delete":                    {Kind: Call, Target: "KeyValue.Delete", Shape: AddCtx, Ref: refKV},
	"KeyValue.Purge":                     {Kind: Call, Target: "KeyValue.Purge", Shape: AddCtx, Ref: refKV},
	"KeyValue.PurgeDeletes":              {Kind: Call, Target: "KeyValue.PurgeDeletes", Shape: AddCtx, Ref: refKV},
	"KeyValue.Watch":                     {Kind: Call, Target: "KeyValue.Watch", Shape: AddCtx, Ref: refKV},
	"KeyValue.WatchAll":                  {Kind: Call, Target: "KeyValue.WatchAll", Shape: AddCtx, Ref: refKV},
	"KeyValue.WatchFiltered":             {Kind: Call, Target: "KeyValue.WatchFiltered", Shape: AddCtx, Ref: refKV},
	"KeyValue.Keys":                      {Kind: Call, Target: "KeyValue.Keys", Shape: AddCtx, Ref: refKV},
	"KeyValue.ListKeys":                  {Kind: Call, Target: "KeyValue.ListKeys", Shape: AddCtx, Ref: refKV},
	"KeyValue.History":                   {Kind: Call, Target: "KeyValue.History", Shape: AddCtx, Ref: refKV},
	"KeyValue.Status":                    {Kind: Call, Target: "KeyValue.Status", Shape: AddCtx, Ref: refKV},
	"KeyValueEntry":                      {Kind: Rename, Target: "KeyValueEntry", Ref: refKV},
	"KeyValueEntry.Bucket":               {Kind: Same, Target: "KeyValueEntry.Bucket", Ref: refKV},
	"KeyValueEntry.Key":                  {Kind: Same, Target: "KeyValueEntry.Key", Ref: refKV},
	"KeyValueEntry.Value":                {Kind: Same, Target: "KeyValueEntry.Value", Ref: refKV},
	"KeyValueEntry.Revision":             {Kind: Same, Target: "KeyValueEntry.Revision", Ref: refKV},
	"KeyValueEntry.Created":              {Kind: Same, Target: "KeyValueEntry.Created", Ref: refKV},
	"KeyValueEntry.Delta":                {Kind: Same, Target: "KeyValueEntry.Delta", Ref: refKV},
	"KeyValueEntry.Operation":            {Kind: Same, Target: "KeyValueEntry.Operation", Ref: refKV},
	"KeyValueOp":                         {Kind: Rename, Target: "KeyValueOp", Ref: refKV},
	"KeyValueOp.String":                  {Kind: Same, Target: "KeyValueOp.String", Ref: refKV},
	"KeyValueStatus":                     {Kind: Rename, Target: "KeyValueStatus", Ref: refKV},
	"KeyValueStatus.Bucket":              {Kind: Same, Target: "KeyValueStatus.Bucket", Ref: refKV},
	"KeyValueStatus.Values":              {Kind: Same, Target: "KeyValueStatus.Values", Ref: refKV},
	"KeyValueStatus.History":             {Kind: Same, Target: "KeyValueStatus.History", Ref: refKV},
	"KeyValueStatus.TTL":                 {Kind: Same, Target: "KeyValueStatus.TTL", Ref: refKV},
	"KeyValueStatus.BackingStore":        {Kind: Same, Target: "KeyValueStatus.BackingStore", Ref: refKV},
	"KeyValueStatus.Bytes":               {Kind: Same, Target: "KeyValueStatus.Bytes", Ref: refKV},
	"KeyValueStatus.IsCompressed":        {Kind: Same, Target: "KeyValueStatus.IsCompressed", Ref: refKV},
	"KeyValueStatus.Config":              {Kind: Same, Target: "KeyValueStatus.Config", Ref: refKV},
	"KeyValueBucketStatus":               {Kind: Rename, Target: "KeyValueBucketStatus", Ref: refKV},
	"KeyValueBucketStatus.Bucket":        {Kind: Same, Target: "KeyValueBucketStatus.Bucket", Ref: refKV},
	"KeyValueBucketStatus.Values":        {Kind: Same, Target: "KeyValueBucketStatus.Values", Ref: refKV},
	"KeyValueBucketStatus.History":       {Kind: Same, Target: "KeyValueBucketStatus.History", Ref: refKV},
	"KeyValueBucketStatus.TTL":           {Kind: Same, Target: "KeyValueBucketStatus.TTL", Ref: refKV},
	"KeyValueBucketStatus.BackingStore":  {Kind: Same, Target: "KeyValueBucketStatus.BackingStore", Ref: refKV},
	"KeyValueBucketStatus.Bytes":         {Kind: Same, Target: "KeyValueBucketStatus.Bytes", Ref: refKV},
	"KeyValueBucketStatus.IsCompressed":  {Kind: Same, Target: "KeyValueBucketStatus.IsCompressed", Ref: refKV},
	"KeyValueBucketStatus.Config":        {Kind: Same, Target: "KeyValueBucketStatus.Config", Ref: refKV},
	"KeyValueBucketStatus.StreamInfo":    {Kind: Same, Target: "KeyValueBucketStatus.StreamInfo", Ref: refKV},
	"KeyWatcher":                         {Kind: Rename, Target: "KeyWatcher", Ref: refKV},
	"KeyWatcher.Updates":                 {Kind: Same, Target: "KeyWatcher.Updates", Ref: refKV},
	"KeyWatcher.Stop":                    {Kind: Same, Target: "KeyWatcher.Stop", Ref: refKV},
	"KeyWatcher.Context":                 {Kind: Unmapped, Note: "jetstream.KeyWatcher does not expose the context it was created with; keep the context you passed to Watch"},
	"KeyWatcher.Error":                   {Kind: Unmapped, Note: "jetstream.KeyWatcher has only Updates and Stop; it has no Error method"},
	"KeyLister":                          {Kind: Rename, Target: "KeyLister", Ref: refKV},
	"KeyLister.Keys":                     {Kind: Same, Target: "KeyLister.Keys", Ref: refKV},
	"KeyLister.Stop":                     {Kind: Same, Target: "KeyLister.Stop", Ref: refKV},
	"KeyLister.Error":                    {Kind: Unmapped, Note: "jetstream.KeyLister has only Keys and Stop, and Stop returns an error; it has no Error method"},
	"WatchOpt":                           {Kind: Rename, Target: "WatchOpt", Ref: refKV},
	"IgnoreDeletes":                      {Kind: Option, Target: "IgnoreDeletes", Ref: refKV},
	"IncludeHistory":                     {Kind: Option, Target: "IncludeHistory", Ref: refKV},
	"UpdatesOnly":                        {Kind: Option, Target: "UpdatesOnly", Ref: refKV},
	"MetaOnly":                           {Kind: Option, Target: "MetaOnly", Ref: refKV},
	"DeleteOpt":                          {Kind: BySite, Target: "KVDeleteOpt", Ref: refKV, Note: "delete options map to jetstream KVDeleteOpt"},
	"LastRevision":                       {Kind: Option, Target: "LastRevision", Ref: refKV},
	"PurgeOpt":                           {Kind: BySite, Target: "KVPurgeOpt", Ref: refKV, Note: "purge-deletes options map to jetstream KVPurgeOpt"},
	"DeleteMarkersOlderThan":             {Kind: Option, Target: "DeleteMarkersOlderThan", Ref: refKV},

	// Object store.
	"ObjectStoreManager":                   {Kind: Rename, Target: "ObjectStoreManager", Ref: refObjMgmt},
	"ObjectStoreManager.ObjectStore":       {Kind: Call, Target: "JetStream.ObjectStore", Shape: AddCtx, Ref: refObjMgmt},
	"ObjectStoreManager.CreateObjectStore": {Kind: Call, Target: "JetStream.CreateObjectStore", Shape: AddCtx | PtrToValue, Ref: refObjMgmt},
	"ObjectStoreManager.DeleteObjectStore": {Kind: Call, Target: "JetStream.DeleteObjectStore", Shape: AddCtx, Ref: refObjMgmt},
	"ObjectStoreManager.ObjectStoreNames":  {Kind: Call, Target: "JetStream.ObjectStoreNames", Shape: AddCtx, Accessor: "Name", Ref: refObjMgmt, Note: noteListerErr},
	"ObjectStoreManager.ObjectStores":      {Kind: Call, Target: "JetStream.ObjectStores", Shape: AddCtx, Accessor: "Status", Ref: refObjMgmt, Note: noteListerErr},
	"ObjectStoreConfig":                    {Kind: Rename, Target: "ObjectStoreConfig", Ref: refObj},
	"ObjectStore":                          {Kind: Rename, Target: "ObjectStore", Ref: refObj},
	"ObjectStore.Put":                      {Kind: Call, Target: "ObjectStore.Put", Shape: AddCtx, Ref: refObj},
	"ObjectStore.PutBytes":                 {Kind: Call, Target: "ObjectStore.PutBytes", Shape: AddCtx, Ref: refObj},
	"ObjectStore.PutString":                {Kind: Call, Target: "ObjectStore.PutString", Shape: AddCtx, Ref: refObj},
	"ObjectStore.PutFile":                  {Kind: Call, Target: "ObjectStore.PutFile", Shape: AddCtx, Ref: refObj},
	"ObjectStore.Get":                      {Kind: Call, Target: "ObjectStore.Get", Shape: AddCtx, Ref: refObj},
	"ObjectStore.GetBytes":                 {Kind: Call, Target: "ObjectStore.GetBytes", Shape: AddCtx, Ref: refObj},
	"ObjectStore.GetString":                {Kind: Call, Target: "ObjectStore.GetString", Shape: AddCtx, Ref: refObj},
	"ObjectStore.GetFile":                  {Kind: Call, Target: "ObjectStore.GetFile", Shape: AddCtx, Ref: refObj},
	"ObjectStore.GetInfo":                  {Kind: Call, Target: "ObjectStore.GetInfo", Shape: AddCtx, Ref: refObj},
	"ObjectStore.UpdateMeta":               {Kind: Call, Target: "ObjectStore.UpdateMeta", Shape: AddCtx, Ref: refObj},
	"ObjectStore.Delete":                   {Kind: Call, Target: "ObjectStore.Delete", Shape: AddCtx, Ref: refObj},
	"ObjectStore.AddLink":                  {Kind: Call, Target: "ObjectStore.AddLink", Shape: AddCtx, Ref: refObj},
	"ObjectStore.AddBucketLink":            {Kind: Call, Target: "ObjectStore.AddBucketLink", Shape: AddCtx, Ref: refObj},
	"ObjectStore.Seal":                     {Kind: Call, Target: "ObjectStore.Seal", Shape: AddCtx, Ref: refObj},
	"ObjectStore.Watch":                    {Kind: Call, Target: "ObjectStore.Watch", Shape: AddCtx, Ref: refObj},
	"ObjectStore.List":                     {Kind: Call, Target: "ObjectStore.List", Shape: AddCtx, Ref: refObj},
	"ObjectStore.Status":                   {Kind: Call, Target: "ObjectStore.Status", Shape: AddCtx, Ref: refObj},
	"ObjectOpt":                            {Kind: BySite, Note: noteContextOnly},
	"GetObjectOpt":                         {Kind: Rename, Target: "GetObjectOpt", Ref: refObj},
	"GetObjectShowDeleted":                 {Kind: Option, Target: "GetObjectShowDeleted", Ref: refObj},
	"GetObjectInfoOpt":                     {Kind: Rename, Target: "GetObjectInfoOpt", Ref: refObj},
	"GetObjectInfoShowDeleted":             {Kind: Option, Target: "GetObjectInfoShowDeleted", Ref: refObj},
	"ListObjectsOpt":                       {Kind: Rename, Target: "ListObjectsOpt", Ref: refObj},
	"ListObjectsShowDeleted":               {Kind: Option, Target: "ListObjectsShowDeleted", Ref: refObj},
	"ObjectInfo":                           {Kind: Rename, Target: "ObjectInfo", Ref: refObj},
	"ObjectMeta":                           {Kind: Rename, Target: "ObjectMeta", Ref: refObj},
	"ObjectMetaOptions":                    {Kind: Rename, Target: "ObjectMetaOptions", Ref: refObj},
	"ObjectLink":                           {Kind: Rename, Target: "ObjectLink", Ref: refObj},
	"ObjectResult":                         {Kind: Rename, Target: "ObjectResult", Ref: refObj},
	"ObjectResult.Info":                    {Kind: Same, Target: "ObjectResult.Info", Ref: refObj},
	"ObjectResult.Error":                   {Kind: Same, Target: "ObjectResult.Error", Ref: refObj},
	"ObjectWatcher":                        {Kind: Rename, Target: "ObjectWatcher", Ref: refObj},
	"ObjectWatcher.Updates":                {Kind: Same, Target: "ObjectWatcher.Updates", Ref: refObj},
	"ObjectWatcher.Stop":                   {Kind: Same, Target: "ObjectWatcher.Stop", Ref: refObj},
	"DecodeObjectDigest":                   {Kind: Rename, Target: "DecodeObjectDigest", Ref: refObj},
	"GetObjectDigestValue":                 {Kind: Rename, Target: "GetObjectDigestValue", Ref: refObj},
	"ObjectStoreStatus":                    {Kind: Rename, Target: "ObjectStoreStatus", Ref: refObj},
	"ObjectStoreStatus.Bucket":             {Kind: Same, Target: "ObjectStoreStatus.Bucket", Ref: refObj},
	"ObjectStoreStatus.Description":        {Kind: Same, Target: "ObjectStoreStatus.Description", Ref: refObj},
	"ObjectStoreStatus.TTL":                {Kind: Same, Target: "ObjectStoreStatus.TTL", Ref: refObj},
	"ObjectStoreStatus.Storage":            {Kind: Same, Target: "ObjectStoreStatus.Storage", Ref: refObj},
	"ObjectStoreStatus.Replicas":           {Kind: Same, Target: "ObjectStoreStatus.Replicas", Ref: refObj},
	"ObjectStoreStatus.Sealed":             {Kind: Same, Target: "ObjectStoreStatus.Sealed", Ref: refObj},
	"ObjectStoreStatus.Size":               {Kind: Same, Target: "ObjectStoreStatus.Size", Ref: refObj},
	"ObjectStoreStatus.BackingStore":       {Kind: Same, Target: "ObjectStoreStatus.BackingStore", Ref: refObj},
	"ObjectStoreStatus.Metadata":           {Kind: Same, Target: "ObjectStoreStatus.Metadata", Ref: refObj},
	"ObjectStoreStatus.IsCompressed":       {Kind: Same, Target: "ObjectStoreStatus.IsCompressed", Ref: refObj},
	"ObjectBucketStatus":                   {Kind: Rename, Target: "ObjectBucketStatus", Ref: refObj},
	"ObjectBucketStatus.Bucket":            {Kind: Same, Target: "ObjectBucketStatus.Bucket", Ref: refObj},
	"ObjectBucketStatus.Description":       {Kind: Same, Target: "ObjectBucketStatus.Description", Ref: refObj},
	"ObjectBucketStatus.TTL":               {Kind: Same, Target: "ObjectBucketStatus.TTL", Ref: refObj},
	"ObjectBucketStatus.Storage":           {Kind: Same, Target: "ObjectBucketStatus.Storage", Ref: refObj},
	"ObjectBucketStatus.Replicas":          {Kind: Same, Target: "ObjectBucketStatus.Replicas", Ref: refObj},
	"ObjectBucketStatus.Sealed":            {Kind: Same, Target: "ObjectBucketStatus.Sealed", Ref: refObj},
	"ObjectBucketStatus.Size":              {Kind: Same, Target: "ObjectBucketStatus.Size", Ref: refObj},
	"ObjectBucketStatus.BackingStore":      {Kind: Same, Target: "ObjectBucketStatus.BackingStore", Ref: refObj},
	"ObjectBucketStatus.Metadata":          {Kind: Same, Target: "ObjectBucketStatus.Metadata", Ref: refObj},
	"ObjectBucketStatus.IsCompressed":      {Kind: Same, Target: "ObjectBucketStatus.IsCompressed", Ref: refObj},
	"ObjectBucketStatus.StreamInfo":        {Kind: Same, Target: "ObjectBucketStatus.StreamInfo", Ref: refObj},
}
