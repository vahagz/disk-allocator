package allocator

type Options struct {
	TargetPageSize uint16
	TreePageSize   uint16
	PagerOptions   PagerOptions
}

type PagerOptions struct {
	FileName string
	PageSize int
}
