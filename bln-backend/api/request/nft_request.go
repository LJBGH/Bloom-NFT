package request

type MintRequest struct {
	Name        string `header:"name" binding:"required"`
	Description string `header:"description" binding:"required"`
}
