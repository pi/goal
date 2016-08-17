package atomic

type Int int

func (p *Int) Get() int {
	return LoadInt((*int)(p))
}

func (p *Int) Set(new int) {
	StoreInt((*int)(p), new)
}

func (p *Int) Swap(new int) (old int) {
	return SwapInt((*int)(p), new)
}

func (p *Int) Inc(delta int) {
	AddInt((*int)(p), delta)
}

func (p *Int) Dec(delta int) {
	AddInt((*int)(p), -delta)
}

func (p *Int) CompareAndSwap(old int, new int) (swapped bool) {
	return CompareAndSwapInt((*int)(p), old, new)
}

type Uint uint

func (p *Uint) Get() uint {
	return LoadUint((*uint)(p))
}

func (p *Uint) Set(new uint) {
	StoreUint((*uint)(p), new)
}

func (p *Uint) Swap(new uint) (old uint) {
	return SwapUint((*uint)(p), new)
}

func (p *Uint) Inc(delta uint) {
	AddUint((*uint)(p), delta)
}

func (p *Uint) Dec(delta uint) {
	AddUint((*uint)(p), -delta)
}

func (p *Uint) CompareAndSwap(old uint, new uint) (swapped bool) {
	return CompareAndSwapUint((*uint)(p), old, new)
}
