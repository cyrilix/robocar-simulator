package simulator

type MsgType string

const (
	MsgTypeControl      = MsgType("control")
	MsgTypeTelemetry    = MsgType("telemetry")
	MsgTypeCarConfig    = MsgType("car_config")
	MsgTypeCarLoaded    = MsgType("car_loaded")
	MsgTypeRacerInfo    = MsgType("racer_info")
	MsgTypeCameraConfig = MsgType("cam_config")
)

type Msg struct {
	MsgType MsgType `json:"msg_type"`
}

type TelemetryMsg struct {
	MsgType       MsgType `json:"msg_type"`
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

// ControlMsg is json msg used to control cars. MsgType must be filled with "control"
type ControlMsg struct {
	MsgType  MsgType `json:"msg_type"`
	Steering string  `json:"steering"`
	Throttle string  `json:"throttle"`
	Brake    string  `json:"brake"`
}

type GetSceneNamesMsg struct {
	MsgType   MsgType `json:"msg_type"`
	SceneName string  `json:"scene_name"`
}

type LoadSceneMsg struct {
	MsgType   MsgType `json:"msg_type"`
	SceneName string  `json:"scene_name"`
}

type CarStyle string

const (
	CarConfigBodyStyleDonkey = CarStyle("donkey")
	CarConfigBodyStyleBare   = CarStyle("bare")
	CarConfigBodyStyleCar01  = CarStyle("car01")
)

/*
	# body_style = "donkey" | "bare" | "car01" choice of string
	# body_rgb  = (128, 128, 128) tuple of ints
	# car_name = "string less than 64 char"
*/
type CarConfigMsg struct {
	MsgType   MsgType  `json:"msg_type"`
	BodyStyle CarStyle `json:"body_style"`
	BodyR     string   `json:"body_r"`
	BodyG     string   `json:"body_g"`
	BodyB     string   `json:"body_b"`
	CarName   string   `json:"car_name"`
	FontSize  string   `json:"font_size"`
}

/*
# car_name = "string less than 64 char"
# guid = "some random string"
*/
type RacerBioMsg struct {
	MsgType   MsgType `json:"msg_type"`
	RacerName string  `json:"racer_name"`
	CarName   string  `json:"car_name"`
	Bio       string  `json:"bio"`
	Country   string  `json:"country"`
	Guid      string  `json:"guid"`
}

type CameraImageEnc string

const (
	CameraImageEncJpeg = CameraImageEnc("JPG")
	CameraImageEncPng  = CameraImageEnc("PNG")
	CameraImageEncTga  = CameraImageEnc("TGA")
)

/* Camera config
set any field to Zero to get the default camera setting.
offset_x moves camera left/right
offset_y moves camera up/down
offset_z moves camera forward/back
rot_x will rotate the camera
with fish_eye_x/y == 0.0 then you get no distortion
img_enc can be one of JPG|PNG|TGA
*/
type CamConfigMsg struct {
	MsgType  MsgType        `json:"msg_type"`
	Fov      string         `json:"fov"`
	FishEyeX string         `json:"fish_eye_x"`
	FishEyeY string         `json:"fish_eye_y"`
	ImgW     string         `json:"img_w"`
	ImgH     string         `json:"img_h"`
	ImgD     string         `json:"img_d"`
	ImgEnc   CameraImageEnc `json:"img_enc"`
	OffsetX  string         `json:"offset_x"`
	OffsetY  string         `json:"offset_y"`
	OffsetZ  string         `json:"offset_z"`
	RotX     string         `json:"rot_x"`
}
