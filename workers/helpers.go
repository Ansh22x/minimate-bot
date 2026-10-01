package workers

import "encoding/json"

// jsonUnmarshalFn is the actual json.Unmarshal function, aliased here to avoid import issues.
var jsonUnmarshalFn = json.Unmarshal
