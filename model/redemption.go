package model

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
)

type Redemption struct {
	Id           int            `json:"id"`
	UserId       int            `json:"user_id"`
	Key          string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int            `json:"status" gorm:"default:1"`
	Name         string         `json:"name" gorm:"index"`
	Quota        int            `json:"quota" gorm:"default:100"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
	Count        int            `json:"count" gorm:"-:all"` // only for api request
	UsedUserId   int            `json:"used_user_id"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期
}

func GetAllRedemptions(startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	// 开始事务
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取总数
	err = tx.Model(&Redemption{}).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 获取分页数据
	err = tx.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func SearchRedemptions(keyword string, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Build query based on keyword type
	query := tx.Model(&Redemption{})

	// Only try to convert to ID if the string represents a valid integer
	if id, err := strconv.Atoi(keyword); err == nil {
		query = query.Where("id = ? OR name LIKE ?", id, keyword+"%")
	} else {
		query = query.Where("name LIKE ?", keyword+"%")
	}

	// Get total count
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated data
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func GetRedemptionById(id int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	var err error = nil
	err = DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

func Redeem(key string, userId int) (quota int, err error) {
	if key == "" {
		return 0, errors.New("未提供兑换码")
	}
	if userId == 0 {
		return 0, errors.New("无效的 user id")
	}
	redemption := &Redemption{}

	keyCol := "`key`"
	if common.UsingPostgreSQL {
		keyCol = `"key"`
	}
	common.RandomSleep()
	if common.UsingClickHouse {
		return redeemClickHouse(key, userId)
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(keyCol+" = ?", key).First(redemption).Error
		if err != nil {
			return errors.New("无效的兑换码")
		}
		if redemption.Status != common.RedemptionCodeStatusEnabled {
			return errors.New("该兑换码已被使用")
		}
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
			return errors.New("该兑换码已过期")
		}
		err = tx.Model(&User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", redemption.Quota)).Error
		if err != nil {
			return err
		}
		redemption.RedeemedTime = common.GetTimestamp()
		redemption.Status = common.RedemptionCodeStatusUsed
		redemption.UsedUserId = userId
		err = tx.Save(redemption).Error
		return err
	})
	if err != nil {
		common.SysError("redemption failed: " + err.Error())
		return 0, ErrRedeemFailed
	}
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(redemption.Quota), redemption.Id))
	return redemption.Quota, nil
}

func (redemption *Redemption) Insert() error {
	if common.UsingClickHouse {
		return redemption.insertClickHouse()
	}
	return DB.Create(redemption).Error
}

// insertClickHouse writes the code with a raw INSERT. GORM Create on the
// ClickHouse native protocol can report success without persisting the row,
// so created codes never showed up in the list.
func (redemption *Redemption) insertClickHouse() error {
	if redemption.Id == 0 {
		redemption.Id = int(nextClickHouseTableID(DB, "redemptions"))
	}
	if redemption.Status == 0 {
		redemption.Status = common.RedemptionCodeStatusEnabled
	}
	if redemption.CreatedTime == 0 {
		redemption.CreatedTime = common.GetTimestamp()
	}
	err := DB.Exec(
		"INSERT INTO redemptions (id, user_id, `key`, status, name, quota, created_time, redeemed_time, used_user_id, expired_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		redemption.Id,
		redemption.UserId,
		redemption.Key,
		redemption.Status,
		redemption.Name,
		redemption.Quota,
		redemption.CreatedTime,
		redemption.RedeemedTime,
		redemption.UsedUserId,
		redemption.ExpiredTime,
	).Error
	if err != nil {
		return fmt.Errorf("clickhouse insert redemption %s: %w", redemption.Name, err)
	}
	return nil
}

func (redemption *Redemption) SelectUpdate() error {
	// This can update zero values
	return DB.Model(redemption).Select("redeemed_time", "status").Updates(redemption).Error
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (redemption *Redemption) Update() error {
	var err error
	err = DB.Model(redemption).Select("name", "status", "quota", "redeemed_time", "expired_time").Updates(redemption).Error
	return err
}

func (redemption *Redemption) Delete() error {
	var err error
	err = DB.Delete(redemption).Error
	return err
}

func DeleteRedemptionById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	err = DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

func DeleteInvalidRedemptions() (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where("status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?)", []int{common.RedemptionCodeStatusUsed, common.RedemptionCodeStatusDisabled}, common.RedemptionCodeStatusEnabled, now).Delete(&Redemption{})
	return result.RowsAffected, result.Error
}

var redeemClickHouseMu sync.Mutex

// redeemClickHouse redeems without row locks or GORM Save, neither of which
// work on ClickHouse. A process-wide mutex plus a status re-check stops the
// same code being redeemed twice.
func redeemClickHouse(key string, userId int) (int, error) {
	redeemClickHouseMu.Lock()
	defer redeemClickHouseMu.Unlock()
	redemption := &Redemption{}
	if err := DB.Where("`key` = ?", key).First(redemption).Error; err != nil {
		return 0, errors.New("无效的兑换码")
	}
	if redemption.Status != common.RedemptionCodeStatusEnabled {
		return 0, errors.New("该兑换码已被使用")
	}
	if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
		return 0, errors.New("该兑换码已过期")
	}
	now := common.GetTimestamp()
	if err := DB.Exec("ALTER TABLE redemptions UPDATE status = ?, redeemed_time = ?, used_user_id = ? WHERE id = ? AND status = ? SETTINGS mutations_sync = 2",
		common.RedemptionCodeStatusUsed, now, userId, redemption.Id, common.RedemptionCodeStatusEnabled).Error; err != nil {
		common.SysError("redemption failed: " + err.Error())
		return 0, ErrRedeemFailed
	}
	if err := IncreaseUserQuota(userId, redemption.Quota, true); err != nil {
		common.SysError("redemption quota failed: " + err.Error())
		return 0, ErrRedeemFailed
	}
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(redemption.Quota), redemption.Id))
	return redemption.Quota, nil
}
