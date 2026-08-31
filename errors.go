package nestor

import "fmt"

var ErrNotSupported = fmt.Errorf("nestor: not supported")
var ErrNotFound = fmt.Errorf("nestor: not found")
var ErrInitRequired = fmt.Errorf("nestor: init required")
