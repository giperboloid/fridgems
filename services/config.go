package services

import (
	"encoding/json"
	"sync"

	"bytes"
	"encoding/binary"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/giperboloid/fridgems/entities"
	"github.com/giperboloid/fridgems/pb"
	"github.com/logrus"
	"golang.org/x/net/context"
)

type Configuration struct {
	sync.Mutex
	entities.FridgeConfig
	SubsPool map[string]chan struct{}
}

type ConfigService struct {
	Config     *Configuration
	Center     entities.Server
	Controller *entities.ServicesController
	DevMeta    *entities.DevMeta
	Log        *logrus.Logger
}

func NewConfigService(m *entities.DevMeta, s entities.Server, ctrl *entities.ServicesController,
	l *logrus.Logger) *ConfigService {

	return &ConfigService{
		DevMeta: m,
		Config: &Configuration{
			SubsPool: make(map[string]chan struct{}),
		},
		Center:     s,
		Controller: ctrl,
		Log:        l,
	}
}

func (s *ConfigService) SetInitConfig() {
	pbic := &pb.SetDevInitConfigRequest{
		Time: time.Now().UnixNano(),
		Meta: &pb.DevMeta{
			Type: s.DevMeta.Type,
			Name: s.DevMeta.Name,
			Mac:  s.DevMeta.MAC,
		},
	}

	conn := dial(s.Center)
	defer conn.Close()

	client := pb.NewCenterServiceClient(conn)
	resp, err := client.SetDevInitConfig(context.Background(), pbic)
	if err != nil {
		log.Error("SetInitConfig(): SetDevInitConfig() has failed: ", err)
		panic("init config hasn't been received")
	}

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.BigEndian, resp.Config); err != nil {
		panic(err)
	}

	var fc entities.FridgeConfig
	if err := json.NewDecoder(buf).Decode(&fc); err != nil {
		panic("init config decoding has failed")
	}

	log.Infof("init config: %+v", fc)
	s.updateConfig(&fc)
}

func (s *ConfigService) GetConfig() *Configuration {
	return s.Config
}

func (s *ConfigService) PatchDevConfig(ctx context.Context, r *pb.PatchDevConfigRequest) (*pb.PatchDevConfigResponse, error) {
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.BigEndian, r.Config); err != nil {
		panic(err)
	}

	var fc entities.FridgeConfig
	if err := json.NewDecoder(buf).Decode(&fc); err != nil {
		panic("config patch decoding has failed")
	}

	log.Infof("config patch: %+v", fc)
	s.updateConfig(&fc)
	return &pb.PatchDevConfigResponse{Status: "OK"}, nil
}

func (s *ConfigService) updateConfig(nfc *entities.FridgeConfig) {
	if nfc.TurnedOn && !s.Config.TurnedOn {
		log.Info("fridge is turned on")
	} else if !nfc.TurnedOn && s.Config.TurnedOn {
		log.Info("fridge is turned off")
	}

	//s.Config.TurnedOn = nfc.TurnedOn
	s.Config.TurnedOn = true
	s.Config.CollectFreq = nfc.CollectFreq
	s.Config.SendFreq = nfc.SendFreq

	s.Config.publishPatchedConfig()
}

func (c *Configuration) publishPatchedConfig() {
	for _, v := range c.SubsPool {
		v <- struct{}{}
	}
}

func (c *Configuration) Subscribe(key string, value chan struct{}) {
	c.Mutex.Lock()
	c.SubsPool[key] = value
	c.Mutex.Unlock()
}

func (c *Configuration) GetTurnedOn() bool {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	return c.TurnedOn
}
func (c *Configuration) SetTurnedOn(b bool) {
	c.Mutex.Lock()
	c.TurnedOn = b
	c.Mutex.Unlock()
}

func (c *Configuration) GetCollectFreq() int64 {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	return c.CollectFreq
}
func (c *Configuration) SetCollectFreq(cf int64) {
	c.Mutex.Lock()
	c.CollectFreq = cf
	c.Mutex.Unlock()
}

func (c *Configuration) GetSendFreq() int64 {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	return c.SendFreq
}
func (c *Configuration) SetSendFreq(sf int64) {
	c.Mutex.Lock()
	c.SendFreq = sf
	c.Mutex.Unlock()
}