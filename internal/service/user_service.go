package service

import (
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/pkg/errors"
	bizerrors "Qavor/pkg/errors"
	"Qavor/pkg/response"
	"Qavor/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

// userService 用户服务实现
type userService struct {
	userRepo repository.UserRepository
}

// NewUserService 创建用户服务
func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{
		userRepo: userRepo,
	}
}

// Register 用户注册
// 自动生成 UID，格式为: usr_<UUID>
func (s *userService) Register(req *request.RegisterRequest) error {
	// 检查邮箱是否存在
	exists, err := s.userRepo.ExistsByEmail(req.Email)
	if err != nil {
		return err
	}
	if exists {
		return bizerrors.New(errors.CodeUserAlreadyExists, "邮箱已被注册")
	}

	// 生成 UUID 并构建 UID
	uuid, err := utils.GenerateUUID()
	if err != nil {
		return bizerrors.New(errors.CodeInternalError, "生成用户标识失败")
	}
	uid := "usr_" + uuid

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 创建用户
	user := &entity.User{
		Nickname: req.Nickname,
		UID:      uid,
		Password: string(hashedPassword),
		Email:    req.Email,
		Status:   1,      // 默认正常
		Role:     "user", // 默认角色
	}

	if err := s.userRepo.Create(user); err != nil {
		return err
	}

	return nil
}

// GetUserByUID 根据 UID 获取用户
func (s *userService) GetUserByUID(uid string) (*entity.User, error) {
	user, err := s.userRepo.FindByUID(uid)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrUserNotFound
	}
	return user, nil
}

// UpdateUser 更新用户信息
func (s *userService) UpdateUser(uid string, req *request.UpdateUserRequest) error {
	user, err := s.GetUserByUID(uid)
	if err != nil {
		return err
	}

	// 更新字段
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}

	return s.userRepo.Update(user)
}

// ChangePassword 修改密码
func (s *userService) ChangePassword(uid string, req *request.ChangePasswordRequest) error {
	user, err := s.GetUserByUID(uid)
	if err != nil {
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return bizerrors.ErrInvalidCredentials
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)
	return s.userRepo.Update(user)
}

// GetUserResponse 获取用户响应
func (s *userService) GetUserResponse(user *entity.User) *dto.UserResponse {
	return &dto.UserResponse{
		Nickname:  user.Nickname,
		UID:       user.UID,
		Email:     user.Email,
		Avatar:    user.Avatar,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// ListUsers 分页获取用户列表
func (s *userService) ListUsers(req *request.UserListRequest) (*response.PageResponse, error) {
	// 参数标准化
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.Size
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	// 计算偏移量
	offset := (page - 1) * size

	// 查询数据
	users, total, err := s.userRepo.List(offset, size)
	if err != nil {
		return nil, err
	}

	// 转换为响应 DTO
	list := make([]*dto.UserResponse, 0, len(users))
	for _, user := range users {
		list = append(list, s.GetUserResponse(user))
	}

	return response.NewPageResponse(list, total, page, size), nil
}
