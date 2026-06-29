DATABASES = {
    "default": {
        "ENGINE": "django.db.backends.postgresql",
        "NAME": "__TEMPLATE__POSTGRES_DATABASE__数据库名__DEFAULT__mydb__",
        "USER": "__TEMPLATE__DATABASE_USERNAME__数据库用户__DEFAULT__postgres__",
        "PASSWORD": "${DATABASE_PASSWORD}",
        "HOST": "__TEMPLATE__POSTGRES_HOST__PostgreSQL 地址__DEFAULT__postgresql__",
        "PORT": "__TEMPLATE__POSTGRES_PORT__PostgreSQL 端口__DEFAULT__5432__",
        "CONN_MAX_AGE": 600,
    }
}

CACHES = {
    "default": {
        "BACKEND": "django_redis.cache.RedisCache",
        "LOCATION": "redis://__TEMPLATE__REDIS_HOST__Redis 地址__DEFAULT__redis-master__:__TEMPLATE__REDIS_PORT__Redis 端口__DEFAULT__6379__/__TEMPLATE__REDIS_DB__Redis 数据库__DEFAULT__1__",
        "OPTIONS": {
            "CLIENT_CLASS": "django_redis.client.DefaultClient",
            "PASSWORD": "${REDIS_PASSWORD}",
            "CONNECTION_POOL_KWARGS": {"max_connections": 100},
        },
    }
}
