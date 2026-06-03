package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/flagmate/suricata-ctf/backend/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Store struct {
	DB    *gorm.DB
	Redis *redis.Client
	ctx   context.Context
}

func New(dsn string, redisURL string) (*Store, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	if err := db.AutoMigrate(&models.Service{}, &models.Pattern{}, &models.Flow{}); err != nil {
		return nil, fmt.Errorf("failed to auto migrate: %w", err)
	}

	// Parse redis://host:port/db format
	addr := redisURL
	dbNum := 0
	password := os.Getenv("REDIS_PASSWORD")
	
	if len(redisURL) > 0 {
		// Handle redis://host:port/db format
		if redisURL[:8] == "redis://" {
			redisURL = redisURL[8:]
		}
		parts := strings.Split(redisURL, "/")
		addr = parts[0]
		if len(parts) > 1 {
			dbNum, _ = strconv.Atoi(parts[1])
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		DB:       dbNum,
		Password: password,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Redis not available: %v\n", err)
	}

	return &Store{
		DB:    db,
		Redis: rdb,
		ctx:   ctx,
	}, nil
}

func (s *Store) CreateService(name string, port int, protocol string) (*models.Service, error) {
	svc := models.Service{
		Name:      name,
		Port:      port,
		Protocol:  protocol,
		CreatedAt: time.Now(),
	}
	if err := s.DB.Create(&svc).Error; err != nil {
		return nil, err
	}
	return &svc, nil
}

func (s *Store) ListServices() ([]models.Service, error) {
	var services []models.Service
	err := s.DB.Order("port asc").Find(&services).Error
	return services, err
}

func (s *Store) DeleteService(id int) error {
	return s.DB.Delete(&models.Service{}, id).Error
}

func (s *Store) CreatePattern(pattern string, description string, mode string) (*models.Pattern, error) {
	p := models.Pattern{
		Pattern:     pattern,
		Description: description,
		Mode:        mode,
		Active:      true,
		MatchCount:  0,
		CreatedAt:   time.Now(),
	}
	if err := s.DB.Create(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) ListPatterns() ([]models.Pattern, error) {
	var patterns []models.Pattern
	err := s.DB.Order("id asc").Find(&patterns).Error
	return patterns, err
}

func (s *Store) DeletePattern(id int) error {
	return s.DB.Delete(&models.Pattern{}, id).Error
}

func (s *Store) SaveFlow(flow models.Flow) error {
	if err := s.DB.Create(&flow).Error; err != nil {
		return err
	}
	s.Redis.XAdd(s.ctx, &redis.XAddArgs{
		Stream: "flows:new",
		Values: map[string]interface{}{
			"id":            flow.ID.String(),
			"service_id":    flow.ServiceID,
			"direction":     flow.Direction,
			"hash":          flow.Hash,
			"stable":        flow.Stable,
			"checker":       flow.Checker,
			"banned":        flow.Banned,
			"response_code": flow.ResponseCode,
			"src_ip":        flow.SrcIP,
			"dst_ip":        flow.DstIP,
			"src_port":      flow.SrcPort,
			"dst_port":      flow.DstPort,
			"proto":         flow.Proto,
			"created_at":    flow.CreatedAt.Format(time.RFC3339),
		},
	})
	return nil
}

func (s *Store) GetFlow(id uuid.UUID) (*models.Flow, error) {
	var flow models.Flow
	err := s.DB.First(&flow, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &flow, nil
}

func (s *Store) ListFlows(page, size int, search string) ([]models.Flow, int64, error) {
	var flows []models.Flow
	var total int64

	query := s.DB.Model(&models.Flow{})
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("direction LIKE ? OR hash LIKE ? OR src_ip LIKE ? OR dst_ip LIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern)
	}

	query.Count(&total)
	err := query.Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&flows).Error
	return flows, total, err
}

func (s *Store) UpdateFlowLabel(id uuid.UUID, checker bool) error {
	return s.DB.Model(&models.Flow{}).Where("id = ?", id).Update("checker", checker).Error
}

func (s *Store) FlagFlowAsBanned(id uuid.UUID) error {
	return s.DB.Model(&models.Flow{}).Where("id = ?", id).Update("banned", true).Error
}

func (s *Store) IncrementHashCount(hash string) int64 {
	key := "flow_hashes"
	s.Redis.ZAdd(s.ctx, key, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: hash,
	})
	s.Redis.ZIncrBy(s.ctx, key, 1, hash)
	count, _ := s.Redis.ZScore(s.ctx, key, hash).Result()
	return int64(count)
}

func (s *Store) GetTopFlowGroups(top int) ([]models.FlowGroup, error) {
	groups, err := s.Redis.ZRevRangeWithScores(s.ctx, "flow_hashes", 0, int64(top-1)).Result()
	if err != nil {
		return nil, err
	}

	var result []models.FlowGroup
	for _, g := range groups {
		hash := g.Member.(string)
		count := int(g.Score)

		var flow models.Flow
		s.DB.Where("hash = ?", hash).Order("created_at desc").First(&flow)

		result = append(result, models.FlowGroup{
			Hash:          hash,
			Count:         count,
			ExampleFlowID: flow.ID,
			FirstSeen:     flow.CreatedAt,
			LastSeen:      flow.CreatedAt,
		})
	}
	return result, nil
}

func (s *Store) SetMirroringConfig(config models.MirroringConfig) error {
	data, _ := json.Marshal(config)
	return s.Redis.Set(s.ctx, "mirroring:config", data, 0).Err()
}

func (s *Store) GetMirroringConfig() (*models.MirroringConfig, error) {
	data, err := s.Redis.Get(s.ctx, "mirroring:config").Bytes()
	if err != nil {
		return nil, err
	}
	var config models.MirroringConfig
	err = json.Unmarshal(data, &config)
	return &config, err
}

func (s *Store) GetFlowsByHash(hash string) ([]models.Flow, error) {
	var flows []models.Flow
	err := s.DB.Where("hash = ?", hash).Order("created_at desc").Find(&flows).Error
	return flows, err
}

func (s *Store) GetUniqueWords(flowID uuid.UUID) ([]string, error) {
	var flow models.Flow
	if err := s.DB.First(&flow, "id = ?", flowID).Error; err != nil {
		return nil, err
	}

	var checkerFlows []models.Flow
	s.DB.Where("checker = ? AND id != ?", true, flowID).Find(&checkerFlows)

	targetWords := extractWords(flow.NormPayload)
	checkerWords := make(map[string]bool)
	for _, cf := range checkerFlows {
		for w := range extractWords(cf.NormPayload) {
			checkerWords[w] = true
		}
	}

	var unique []string
	for w := range targetWords {
		if !checkerWords[w] {
			unique = append(unique, w)
		}
	}
	return unique, nil
}

func extractWords(payload string) map[string]bool {
	words := make(map[string]bool)
	current := strings.Builder{}
	for _, r := range payload {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			current.WriteRune(r)
		} else {
			if current.Len() > 2 {
				words[strings.ToLower(current.String())] = true
			}
			current.Reset()
		}
	}
	if current.Len() > 2 {
		words[strings.ToLower(current.String())] = true
	}
	return words
}

func (s *Store) UnbanFlow(id uuid.UUID) error {
	return s.DB.Model(&models.Flow{}).Where("id = ?", id).Update("banned", false).Error
}
