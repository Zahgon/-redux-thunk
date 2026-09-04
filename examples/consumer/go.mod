// A deliberately separate module: it proves the library is consumable from
// outside its own module, which is the Go equivalent of the upstream
// `test-published-artifact` CI job.
module example.com/redux-thunk-go-consumer

go 1.22

require github.com/reduxjs/redux-thunk-go v0.0.0

replace github.com/reduxjs/redux-thunk-go => ../..
