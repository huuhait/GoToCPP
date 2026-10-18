package db

import (
	"example.com/vars/cfg"
	"example.com/vars/util"
)

var Conn = util.NewCounter()

var Z = util.A + cfg.Max

func Use() int { return Conn.Inc() + util.CC }

type Config = cfg.Opt

type Level cfg.Opt

type Meters int

func (m Meters) Double() Meters { return m * 2 }
