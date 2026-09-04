package request

type CreatePostReq struct {
	PostCode string `json:"post_code" form:"post_code" binding:"required"`
	PostName string `json:"post_name" form:"post_name" binding:"required"`
	DeptId   int64  `json:"dept_id" form:"dept_id" binding:"required"`
}

type GetPostReq struct {
	ID       int64  `json:"id" form:"id"`
	PostCode string `json:"post_code" form:"post_code"`
	PostName string `json:"post_name" form:"post_name"`
}

type ListPostReq struct {
	DeptId   int64 `json:"dept_id" form:"dept_id"`
	Page     int   `json:"page" form:"page"`
	PageSize int   `json:"pageSize" form:"pageSize"`
}
