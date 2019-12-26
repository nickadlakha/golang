package jukeboxstruct

var (
	AmqpHost = "localhost"
	AmqpPort = 5672
)

type Stype int

const (
	_ Stype = iota
	Stream
	Sink
)

type Mp3Info_ struct {
	Rate     int
	Channels int
	Swidth   int
}

type JukeboxStruct struct {
	AmqpHost     string
	FlagMaster   bool
	PlayerR      bool
	Sync         Stype
	PcmData      interface{}
	Mp3Data      interface{}
	Connection   interface{}
	Mp3Info      Mp3Info_
	SkipSinkHost bool
}
