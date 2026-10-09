package api

import "io"

type StringOrBytes interface {
	string | []byte
}

func WriteMust[T StringOrBytes](w io.Writer, b T) {
	if _, err := w.Write([]byte(b)); err != nil {
		panic(err)
	}
}
