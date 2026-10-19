package modgraph

import (
	"os"
)

func statDir(p string) (bool, error) {
	st, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	return st.IsDir(), nil
}
