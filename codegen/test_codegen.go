package codegen

import "context"

type It interface {
	InterfaceTest1(a int, b string) error
	InterfaceTest2() (string, int)
	InterfaceTest3(x any, y []any, z [][]string) ([]string, any, error)
	InterfaceTest4()
	InterfaceTest5() error
	InterfaceTest6(ctx context.Context, key string) (string, error)
	It2
}

type It2 interface {
	InterfaceTestA() error
	InterfaceTestB(x int) (int, error)
}

type St struct {
	id int
	n  string
}

// some St methods
func (s *St) StructTest1(a int, b string) error {
	return nil
}

func (s *St) StructTest2() (string, int) {
	return s.n, s.id
}

func (s *St) StructTest3(x any, y []any, z [][]string) ([]string, any, error) {
	return []string{}, nil, nil
}

func (s *St) StructTest4() {
	// do nothing
}

func (s St) StructTest5() error {
	return nil
}

func (s St) StructTest6(x any, y []any, z [][]string) error {
	return nil
}
