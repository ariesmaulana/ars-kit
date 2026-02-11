# General & Standard Conventions

## Language
- All our code and documentation should be written in English.
- Naming conventions for variable, function, and package names should follow the recommendations from the Go team.
- For most of the case use lower camel case: e.g: memberId, userId.

## Error Handling
- No swallow error.
- Every error handling must be following with a log message.

## Logging
- Every log message must be following with a timestamp.
- Every log message must be following with a log level.
- Every log message must be following with a log category.
- Every log message must be following with a log traceId

## General Code Style
We split the code into multiple files for each "domain", every file should have a clear responsibility.
- data.go: this file contains all the data related functions.
example:
```go

// Enum
type CategoryPlan string

const (
	CPIncome  CategoryPlan = "income"
	CPExpense CategoryPlan = "expense"
)

type Plan struct {
	Id        int
	MemberId  int
	Month     int
	Year      int
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

service.go: this file contains all logic, preferable are heavy lifting logic will be inside service layer. Service layer commonly using this pattern: 
- validation 
- open connection
- actual processing
- response
example:
```go
func (s *service) Register(ctx context.Context, input *RegisterInput) *RegisterOutput {
	resp := &RegisterOutput{TraceId: input.TraceId}

	if input.Username == "" {
		log.Warn().Msg("Username empty")
		resp.Message = "Username is mandatory"
		return resp
	}

	if len(input.Username) < 5 {
		log.Warn().Msg("Username too short")
		resp.Message = "Username must be at least 5 characters long"
		return resp
	}
	if input.Email == "" {
		log.Warn().Msg("Email empty")
		resp.Message = "Email is mandatory"
		return resp
	}

	err := validateEmail(input.Email)
	if err != nil {
		log.Warn().Msg("Invalid email")
		resp.Message = "Invalid email"
		return resp
	}

	if input.Password == "" {
		log.Warn().Msg("Password empty")
		resp.Message = "Password is mandatory"
		return resp
	}

	if len(input.Password) < 7 {
		log.Warn().Msg("Password too short")
		resp.Message = "Password must be at least 7 characters long"
		return resp
	}

	if input.FullName == "" {
		log.Warn().Msg("FullName empty")
		resp.Message = "FullName is mandatory"
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		return resp
	}
	defer db.Rollback()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to hash password")
		return resp
	}

	insertedId, err := db.InsertUser(ctx, input.Username, input.Email, input.FullName, string(hashedPassword))
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to insert user")
		return resp
	}

	data, err := db.GetUserById(ctx, insertedId)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to get user")
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Str("username", input.Username).
		Str("email", input.Email).
		Msg("User registered successfully")

	resp.Success = true
	resp.Message = "User registered successfully"
	resp.User = data
	return resp
}

storage.go: This contains all function related to sql query.

handler.go: This contains all function related to http request. Handler will act only as bridge to service layer, no need to put unnecessary validation because all logic will be handled by service layer.

```
## Test

Follow testing/README.md for setup the test
