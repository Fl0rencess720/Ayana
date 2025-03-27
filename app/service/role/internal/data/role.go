package data

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Fl0rencess720/Wittgenstein/app/service/role/internal/biz"
	"github.com/Fl0rencess720/Wittgenstein/pkgs/utils"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
)

type roleRepo struct {
	data *Data
	log  *log.Helper
}

func NewRoleRepo(data *Data, logger log.Logger) biz.RoleRepo {
	return &roleRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *roleRepo) CreateRole(ctx context.Context, phone string, role biz.Role) (string, error) {
	uid, err := utils.GetSnowflakeID(0)
	if err != nil {
		return "", err
	}
	role.Uid = uid
	role.Phone = phone
	if err := r.data.mysqlClient.Create(&role).Error; err != nil {
		return "", err
	}
	return uid, nil
}

func (r *roleRepo) GetRoles(ctx context.Context, phone string) ([]biz.Role, error) {
	roles := []biz.Role{}
	if err := r.data.mysqlClient.Where("phone = ?", phone).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *roleRepo) DeleteRole(ctx context.Context, uid string) error {
	if err := r.data.mysqlClient.Delete(&biz.Role{}, "uid = ?", uid).Error; err != nil {
		return err
	}
	return nil
}

func (r *roleRepo) RolesToRedis(ctx context.Context, phone string, roles []biz.Role) error {
	serializedRole, err := json.Marshal(roles)
	if err != nil {
		return err
	}
	return r.data.redisClient.Set(ctx, "roles:"+phone, serializedRole, 3*24*time.Hour).Err()
}

func (r *roleRepo) GetSeminarRolesFromRedis(ctx context.Context, uids []string) ([]biz.Role, []string, error) {
	roles, notFounds, err := fetchValuesAndNotFound(r.data.redisClient, uids)
	if err != nil {
		return nil, nil, err
	}
	return roles, notFounds, nil
}

func fetchValuesAndNotFound(rdb *redis.Client, uids []string) ([]biz.Role, []string, error) {
	ctx := context.Background()
	type result struct {
		element string
		keys    []string
		err     error
	}

	concurrency := 5
	chElement := make(chan string, len(uids))
	chResult := make(chan result, len(uids))

	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range chElement {
				pattern := "*" + s + "*"
				keys, err := scanKeys(rdb, ctx, pattern)
				chResult <- result{s, keys, err}
			}
		}()
	}

	go func() {
		for _, u := range uids {
			chElement <- u
		}
		close(chElement)
	}()

	go func() {
		wg.Wait()
		close(chResult)
	}()

	foundKeys := make(map[string]struct{})
	notFound := make([]string, 0)

	for res := range chResult {
		if res.err != nil {
			return nil, nil, fmt.Errorf("error querying keys for %s: %v", res.element, res.err)
		}
		if len(res.keys) == 0 {
			notFound = append(notFound, res.element)
		} else {
			for _, key := range res.keys {
				foundKeys[key] = struct{}{}
			}
		}
	}

	keysSlice := make([]string, 0, len(foundKeys))
	for key := range foundKeys {
		keysSlice = append(keysSlice, key)
	}

	values, err := rdb.MGet(ctx, keysSlice...).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("MGet error: %v", err)
	}

	resultValues := make([]biz.Role, 0, len(values))
	for _, val := range values {
		if val == nil {
			continue
		}
		if r, ok := val.(biz.Role); ok {
			resultValues = append(resultValues, r)
		}
	}

	return resultValues, notFound, nil
}

func scanKeys(rdb *redis.Client, ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	var cursor uint64
	var err error

	for {
		var batchKeys []string
		batchKeys, cursor, err = rdb.Scan(ctx, cursor, pattern, 0).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batchKeys...)
		if cursor == 0 {
			break
		}
	}

	return keys, nil
}
