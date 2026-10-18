package k


const (
	Big      = 1e400 // an untyped constant that is no float64
	Huge     = 1 << 100
	Small    = 1e-300
	Pi2      = 3.141592653589793 * 2
	F32      float32 = 0.1
	U8       uint8   = 200
	Neg      int8    = -128
	Mask             = 0xFFFFFFFFFFFFFFFF
	Str              = "multi\nline \"quoted\" \x00 nul"
	Ch               = 'x'
	Flag             = true
	Cplx             = 1 + 2i
	Sh               = Huge >> 98
	Dur      Duration = 3 * Second
	Second   Duration = 1000
	Unicode          = "héllo, 世界"
)

type Duration int64

func (d Duration) Millis() int64 { return int64(d) }

func Ratio() float64 { return Big / Big * 0.5 }
