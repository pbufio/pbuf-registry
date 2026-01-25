#!/usr/bin/env python3

import os
import sys
import time

import psycopg


def env(name: str, default: str = "") -> str:
    val = os.getenv(name)
    if val is None or val == "":
        return default
    return val


def wait_for_db(dsn: str, timeout_seconds: int) -> None:
    deadline = time.time() + timeout_seconds
    last_err: Exception | None = None

    while time.time() < deadline:
        try:
            with psycopg.connect(dsn, autocommit=True) as conn:
                with conn.cursor() as cur:
                    cur.execute("SELECT 1")
                    cur.fetchone()
            return
        except Exception as e:  # noqa: BLE001
            last_err = e
            time.sleep(0.5)

    raise RuntimeError(f"database did not become ready in {timeout_seconds}s: {last_err}")


def require_schema(dsn: str, timeout_seconds: int) -> None:
    deadline = time.time() + timeout_seconds
    last_err: Exception | None = None

    while time.time() < deadline:
        try:
            with psycopg.connect(dsn, autocommit=True) as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        SELECT
                          EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'users')
                          AND EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'acl')
                        """
                    )
                    ok = cur.fetchone()[0]
                    if ok:
                        return
        except Exception as e:  # noqa: BLE001
            last_err = e

        time.sleep(0.5)

    raise RuntimeError(f"schema (users/acl) did not become ready in {timeout_seconds}s: {last_err}")


def upsert_user(conn: psycopg.Connection, name: str, token: str, user_type: str) -> str:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO users (name, token, type, is_active, created_at, updated_at)
            VALUES (%s, crypt(%s, gen_salt('bf')), %s, true, NOW(), NOW())
            ON CONFLICT (name) DO UPDATE
              SET token = crypt(%s, gen_salt('bf')),
                  type = EXCLUDED.type,
                  is_active = true,
                  updated_at = NOW()
            RETURNING id
            """,
            (name, token, user_type, token),
        )
        return str(cur.fetchone()[0])


def upsert_acl(conn: psycopg.Connection, user_id: str, module_name: str, permission: str) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO acl (user_id, module_name, permission, created_at)
            VALUES (%s, %s, %s, NOW())
            ON CONFLICT (user_id, module_name) DO UPDATE
              SET permission = EXCLUDED.permission
            """,
            (user_id, module_name, permission),
        )


def main() -> int:
    dsn = env("DATABASE_DSN") or env("DATA_DATABASE_DSN")
    if not dsn:
        print("DATABASE_DSN (or DATA_DATABASE_DSN) is required", file=sys.stderr)
        return 2

    module_name = env("SEED_MODULE_NAME", "yourorg/hello-proto")
    reader_token = env("SEED_READER_TOKEN", "pbuf_user_dev_reader_token")
    writer_token = env("SEED_WRITER_TOKEN", "pbuf_user_dev_writer_token")
    bot_token = env("SEED_BOT_TOKEN", "pbuf_bot_dev_ci_token")

    wait_for_db(dsn, timeout_seconds=30)
    require_schema(dsn, timeout_seconds=60)

    with psycopg.connect(dsn, autocommit=True) as conn:
        # Seed users/bots
        reader_id = upsert_user(conn, "dev-reader", reader_token, "user")
        writer_id = upsert_user(conn, "dev-writer", writer_token, "user")
        bot_id = upsert_user(conn, "dev-ci-bot", bot_token, "bot")

        # Seed permissions
        upsert_acl(conn, reader_id, "*", "read")
        upsert_acl(conn, writer_id, module_name, "write")
        upsert_acl(conn, bot_id, module_name, "write")

    print("Seed complete.")
    print()
    print("Created/updated actors:")
    print(f"  dev-reader  id={reader_id}  token={reader_token}")
    print(f"  dev-writer  id={writer_id}  token={writer_token}")
    print(f"  dev-ci-bot  id={bot_id}     token={bot_token}")
    print()
    print("Example env for pbuf CLI:")
    print("  export PBUF_REGISTRY_URL=https://localhost:6777")
    print("  export PBUF_REGISTRY_INSECURE=true  # self-signed certs")
    print(f"  export PBUF_REGISTRY_TOKEN={writer_token}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
