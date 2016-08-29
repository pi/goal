package gut

type zeroLocker int

func (zeroLocker) Lock()   {}
func (zeroLocker) Unlock() {}

var ZeroLocker zeroLocker = zeroLocker(0)
