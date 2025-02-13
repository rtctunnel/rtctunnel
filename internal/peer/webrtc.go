package peer

// An RTCDataChannel abstracts an RTCDataChannel
type RTCDataChannel interface {
	Close() error
	Label() string
	OnClose(func())
	OnMessage(func([]byte))
	OnOpen(func())
	Send([]byte) error
}

// An RTCPeerConnection abstracts an RTCPeerConnection
type RTCPeerConnection interface {
	AddICECandidate(string) error
	Close() error
	CreateDataChannel(label string) (RTCDataChannel, error)
	OnDataChannel(func(dc RTCDataChannel))
	OnICECandidate(func(string))
	OnICEConnectionStateChange(func(string))
	CreateAnswer() (string, error)
	CreateOffer() (string, error)
	SetAnswer(answer string) error
	SetOffer(offer string) error
}
