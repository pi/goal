package gut

func Perr(err error) {
	if err != nil {
		panic(err.Error())
	}
}
