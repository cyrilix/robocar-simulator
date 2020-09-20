package simulator

type SimulatorMsg struct {
	MsgType       string  `json:"msg_type"`
	SteeringAngle float64 `json:"steering_angle"`
	Throttle      float64 `json:"throttle"`
	Speed         float64 `json:"speed"`
	Image         []byte  `json:"image"`
	Hit           string  `json:"hit"`
	PosX          float64 `json:"pos_x"`
	PosY          float64 `json:"posy"`
	PosZ          float64 `json:"pos_z"`
	Time          float64 `json:"time"`
	Cte           float64 `json:"cte"`
}
