package repository

// import (
// 	"bloom-nft/api/request"
// 	"bloom-nft/model"

// 	"gorm.io/gorm"
// )

// type ProjectContractRepository struct {
// 	DB *gorm.DB
// }

// func NewProjectContractRepository(db *gorm.DB) *ProjectContractRepository {
// 	return &ProjectContractRepository{DB: db}
// }

// // 根据ID获取销售合约
// func (r *ProjectContractRepository) GetById(id uint) (*model.ProjectContract, error) {
// 	var saleContract model.ProjectContract
// 	if err := r.DB.Where("id = ?", id).First(&saleContract).Error; err != nil {
// 		return nil, err
// 	}
// 	return &saleContract, nil
// }

// // 获取所有销售合约
// func (r *ProjectContractRepository) GetAll() ([]model.ProjectContract, error) {
// 	var saleContracts []model.ProjectContract
// 	if err := r.DB.Find(&saleContracts).Error; err != nil {
// 		return nil, err
// 	}
// 	return saleContracts, nil
// }

// // 更新销售合约
// func (r *ProjectContractRepository) Update(request *request.ProjectContractUpdateRequest) error {
// 	if err := r.DB.Model(&model.ProjectContract{}).Where("id = ?", request.ID).Select(
// 		"SaleContractAddress",
// 		"TokenAddress",
// 		"TokenPriceInPT",
// 		"TotalTokensSold",
// 		"SaleEnd",
// 		"UnlockTime",
// 		"RegistrationTimeStarts",
// 		"RegistrationTimeEnds",
// 		"SaleStart",
// 	).Updates(model.ProjectContract{
// 		SaleContractAddress:    request.SaleAddress,
// 		TokenAddress:           request.SaleToken,
// 		TokenPriceInPT:         request.TokenPriceInPT,
// 		TotalTokensSold:        request.TotalTokens,
// 		SaleEnd:                request.SaleEndTime.Time(),
// 		UnlockTime:             request.UnlockTime.Time(),
// 		RegistrationTimeStarts: request.RegistrationStart.Time(),
// 		RegistrationTimeEnds:   request.RegistrationEnd.Time(),
// 		SaleStart:              request.SaleStartTime.Time(),
// 	}).Error; err != nil {
// 		return err
// 	}
// 	return nil
// }
