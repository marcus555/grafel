package substrate

// Connection-pool db_effect attribution (#5008).
//
// A pooled-connection checkout (deadpool / bb8 / r2d2 / mobc / sqlx::Pool)
// reaches the database to lease a live connection, so the leasing function is
// credited db_read even when the subsequent query runs on the leased handle in
// a callee. Gated on a pool-crate signal so the bare .get()/.acquire() method
// names cannot collide with Option::get / HashMap::get / Mutex::acquire.

import "testing"

// A (happy path): each crate's pool checkout earns db_read on the enclosing fn.
func TestRustConnPoolAcquire_5008(t *testing.T) {
	src := `
use deadpool_postgres::Pool;
use bb8::Pool as Bb8Pool;

impl Db {
    pub async fn deadpool_handler(&self, pool: &Pool) -> Result<(), Error> {
        let client = pool.get().await?;
        client.query("anything", &[]).await?;
        Ok(())
    }
    pub async fn bb8_handler(&self, pool: &Bb8Pool) -> Result<(), Error> {
        let conn = pool.get().await?;
        Ok(())
    }
    pub fn r2d2_handler(&self, db_pool: &r2d2::Pool<C>) -> Result<(), Error> {
        let conn = db_pool.get()?;
        Ok(())
    }
    pub async fn mobc_handler(&self, conn_pool: &mobc::Pool<C>) -> Result<(), Error> {
        let conn = conn_pool.get().await?;
        Ok(())
    }
}
`
	by := groupByEffect(sniffEffectsRust(src))
	mustHave(t, by, EffectDBRead, "deadpool_handler")
	mustHave(t, by, EffectDBRead, "bb8_handler")
	mustHave(t, by, EffectDBRead, "r2d2_handler")
	mustHave(t, by, EffectDBRead, "mobc_handler")
}

// B (negative — no pool crate signal): a bare `cache.get()` / `lock.acquire()`
// in a file WITHOUT any pool crate earns no db_read (false-positive guard).
func TestRustConnPoolNoSignalNoCredit_5008(t *testing.T) {
	src := `
use std::collections::HashMap;

impl Cache {
    pub fn lookup(&self, key: &str) -> Option<&str> {
        let pool = self.map.get(key);
        pool.copied()
    }
}
`
	by := groupByEffect(sniffEffectsRust(src))
	mustNotHave(t, by, EffectDBRead, "lookup")
}

// C (negative — pool crate present but the .get() is on a non-pool receiver):
// a HashMap .get() in a pool-importing file is NOT mis-credited because the
// receiver name does not contain `pool`.
func TestRustConnPoolNonPoolReceiverNoCredit_5008(t *testing.T) {
	src := `
use deadpool_redis::Pool;

impl Svc {
    pub fn read_cfg(&self, settings: &HashMap<String, String>) -> Option<String> {
        let v = settings.get("key");
        v.cloned()
    }
}
`
	by := groupByEffect(sniffEffectsRust(src))
	mustNotHave(t, by, EffectDBRead, "read_cfg")
}
