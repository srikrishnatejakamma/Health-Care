package rediscache

import (
	"context"
	"encoding/base64"
	"errors"
	"healthcare/internal/domain"
	"healthcare/internal/framework/tenancy"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache must be called after resource authorization; cache hits never bypass it.
type Cache struct{client *redis.Client;environment string}
func Open(ctx context.Context,url,environment string)(*Cache,error){if environment==""{return nil,domain.ErrInvalid};options,err:=redis.ParseURL(url);if err!=nil{return nil,err};options.PoolSize=16;options.MinIdleConns=1;options.DialTimeout=5*time.Second;options.ReadTimeout=3*time.Second;options.WriteTimeout=3*time.Second;client:=redis.NewClient(options);if err=client.Ping(ctx).Err();err!=nil{client.Close();return nil,err};return &Cache{client:client,environment:environment},nil}
func(c *Cache)Close()error{return c.client.Close()}
func Key(ctx context.Context,environment,kind,id string)(string,error){s,err:=tenancy.From(ctx);if err!=nil{return "",err};if environment==""||kind==""||id==""{return "",domain.ErrInvalid};parts:=[]string{environment,s.TenantID,kind,id};for i,v:=range parts{parts[i]=base64.RawURLEncoding.EncodeToString([]byte(v))};return strings.Join(parts,":"),nil}
func(c *Cache)Set(ctx context.Context,kind,id string,b []byte,ttl time.Duration)error{if ttl<=0||ttl>15*time.Minute||len(b)>1<<20{return domain.ErrInvalid};key,err:=Key(ctx,c.environment,kind,id);if err!=nil{return err};return c.client.Set(ctx,key,b,ttl).Err()}
func(c *Cache)Get(ctx context.Context,kind,id string)([]byte,error){key,err:=Key(ctx,c.environment,kind,id);if err!=nil{return nil,err};b,err:=c.client.Get(ctx,key).Bytes();if errors.Is(err,redis.Nil){err=domain.ErrNotFound};return b,err}
func(c *Cache)Delete(ctx context.Context,kind,id string)error{key,err:=Key(ctx,c.environment,kind,id);if err!=nil{return err};return c.client.Del(ctx,key).Err()}
