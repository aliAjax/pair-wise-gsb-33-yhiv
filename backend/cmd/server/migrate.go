package main

import (
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

func daysAgo(n int) time.Time     { return time.Now().AddDate(0, 0, -n) }
func daysFromNow(n int) time.Time { return time.Now().AddDate(0, 0, n) }

func migrate(db *gorm.DB) error {
	// AutoMigrate cannot drop columns/indexes: remove the legacy unique index
	// (user_id, plant_species_id) first, because multiple pots of the same
	// species are now allowed. Both the GORM tag name and the init.sql name
	// are handled for existing databases.
	for _, idx := range []string{"idx_garden_user_plant", "uk_garden_user_plant"} {
		if err := dropIndexIfExists(db, "user_gardens", idx); err != nil {
			return err
		}
	}

	// Detect upgrade vs fresh install. A legacy user_gardens table lacks the
	// pot_no column. On legacy databases the new NOT NULL unique columns must
	// be added as nullable, backfilled, then tightened — directly creating the
	// final unique index would fail on duplicate empty pot numbers.
	legacyGarden := tableExists(db, "user_gardens") && !columnExists(db, "user_gardens", "pot_no")
	legacyReminders := tableExists(db, "care_reminders") && !columnExists(db, "care_reminders", "garden_id")
	if legacyGarden || legacyReminders {
		if err := prepareLegacyTables(db, legacyGarden, legacyReminders); err != nil {
			return err
		}
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.PlantSpecies{},
		&model.CareArticle{},
		&model.DiseasePest{},
		&model.Favorite{},
		&model.Question{},
		&model.Answer{},
	); err != nil {
		return err
	}
	// On legacy databases the two redesigned tables are migrated via explicit
	// DDL (their model tags already carry the final NOT NULL unique indexes,
	// which cannot be created before backfill). On fresh installs AutoMigrate
	// creates them directly from the model tags.
	if !legacyGarden {
		if err := db.AutoMigrate(&model.UserGarden{}); err != nil {
			return err
		}
	}
	if !legacyReminders {
		if err := db.AutoMigrate(&model.CareReminder{}); err != nil {
			return err
		}
	}

	if columnExists(db, "user_gardens", "care_reminder_id") {
		if err := db.Exec("ALTER TABLE user_gardens DROP COLUMN care_reminder_id").Error; err != nil {
			return err
		}
	}

	if err := backfillLegacyPots(db); err != nil {
		return err
	}
	if err := backfillLegacyReminders(db); err != nil {
		return err
	}
	if legacyGarden || legacyReminders {
		if err := finalizeLegacyTables(db, legacyGarden, legacyReminders); err != nil {
			return err
		}
	}
	return nil
}

// prepareLegacyTables adds the new columns in a relaxed (nullable / default 0)
// shape so existing rows survive the upgrade before they are backfilled.
func prepareLegacyTables(db *gorm.DB, legacyGarden, legacyReminders bool) error {
	if legacyGarden {
		if err := db.Exec("ALTER TABLE user_gardens ADD COLUMN pot_no VARCHAR(32) NULL AFTER plant_species_id").Error; err != nil {
			return err
		}
		if !columnExists(db, "user_gardens", "status") {
			if err := db.Exec("ALTER TABLE user_gardens ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active'").Error; err != nil {
				return err
			}
		}
	}
	if legacyReminders {
		if err := db.Exec("ALTER TABLE care_reminders ADD COLUMN garden_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER user_id").Error; err != nil {
			return err
		}
	}
	return nil
}

// finalizeLegacyTables creates the final unique indexes and tightens pot_no to
// NOT NULL once every legacy row has been backfilled.
func finalizeLegacyTables(db *gorm.DB, legacyGarden, legacyReminders bool) error {
	if legacyGarden {
		if err := createIndexIfNotExists(db, "user_gardens",
			"CREATE UNIQUE INDEX uk_garden_pot_no ON user_gardens (user_id, pot_no)"); err != nil {
			return err
		}
		if err := createIndexIfNotExists(db, "user_gardens",
			"CREATE INDEX idx_garden_user ON user_gardens (user_id, plant_species_id)"); err != nil {
			return err
		}
		if err := createIndexIfNotExists(db, "user_gardens",
			"CREATE INDEX idx_user_gardens_status ON user_gardens (status)"); err != nil {
			return err
		}
		if err := db.Exec("ALTER TABLE user_gardens MODIFY COLUMN pot_no VARCHAR(32) NOT NULL").Error; err != nil {
			return err
		}
	}
	if legacyReminders {
		if err := createIndexIfNotExists(db, "care_reminders",
			"CREATE UNIQUE INDEX uk_reminder_plan ON care_reminders (user_id, garden_id, task_title, remind_date)"); err != nil {
			return err
		}
		if err := createIndexIfNotExists(db, "care_reminders",
			"CREATE INDEX idx_reminders_garden ON care_reminders (garden_id)"); err != nil {
			return err
		}
		if err := createIndexIfNotExists(db, "care_reminders",
			"CREATE INDEX idx_care_reminders_status ON care_reminders (status)"); err != nil {
			return err
		}
	}
	return nil
}

func tableExists(db *gorm.DB, table string) bool {
	var count int64
	db.Raw("SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", table).Scan(&count)
	return count > 0
}

func columnExists(db *gorm.DB, table, column string) bool {
	var count int64
	db.Raw("SELECT COUNT(1) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?", table, column).Scan(&count)
	return count > 0
}

func createIndexIfNotExists(db *gorm.DB, table, ddl string) error {
	var count int64
	if err := db.Raw(
		"SELECT COUNT(1) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
		table, indexNameFromDDL(ddl),
	).Scan(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return db.Exec(ddl).Error
}

// indexNameFromDDL extracts the index name from a "CREATE [UNIQUE] INDEX x ON"
// statement for idempotency checks.
func indexNameFromDDL(ddl string) string {
	rest := strings.TrimPrefix(ddl, "CREATE UNIQUE ")
	rest = strings.TrimPrefix(rest, "CREATE ")
	rest = strings.TrimPrefix(rest, "INDEX ")
	rest = strings.TrimLeft(rest, " \t")
	end := strings.IndexAny(rest, " (")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// dropIndexIfExists drops an index by name when it exists (MySQL has no
// "DROP INDEX IF EXISTS" before 8.0.29).
func dropIndexIfExists(db *gorm.DB, table, index string) error {
	var count int64
	if err := db.Raw(
		"SELECT COUNT(1) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
		table, index,
	).Scan(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	return db.Exec("ALTER TABLE " + table + " DROP INDEX `" + index + "`").Error
}

// backfillLegacyPots fills pot_no/status for pots created before per-pot
// registration: pot numbers restart at P0001 per user and are assigned by
// takeover date (earliest first).
func backfillLegacyPots(db *gorm.DB) error {
	potFill := `
UPDATE user_gardens ug
JOIN (
  SELECT id, CONCAT('P', LPAD(ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY owned_since ASC, id ASC), 4, '0')) AS new_pot_no
  FROM user_gardens
  WHERE pot_no = '' OR pot_no IS NULL
) t ON t.id = ug.id
SET ug.pot_no = t.new_pot_no`
	if err := db.Exec(potFill).Error; err != nil {
		return err
	}
	return db.Exec(`UPDATE user_gardens SET status = 'active' WHERE status = '' OR status IS NULL`).Error
}

// backfillLegacyReminders attaches reminders created before pot-bound
// reminders to the earliest-taken-over pot of the same species per user.
// Idempotent: only rows without a pot (garden_id = 0) are touched.
func backfillLegacyReminders(db *gorm.DB) error {
	reminderFill := `
UPDATE care_reminders cr
JOIN (
  SELECT cr2.id AS reminder_id, ug.id AS garden_id
  FROM care_reminders cr2
  JOIN (
    SELECT user_id, plant_species_id, MIN(owned_since) AS earliest
    FROM user_gardens
    GROUP BY user_id, plant_species_id
  ) e ON e.user_id = cr2.user_id AND e.plant_species_id = cr2.plant_species_id
  JOIN user_gardens ug
    ON ug.user_id = cr2.user_id
   AND ug.plant_species_id = cr2.plant_species_id
   AND ug.owned_since = e.earliest
  WHERE cr2.garden_id = 0 AND cr2.plant_species_id > 0
) m ON m.reminder_id = cr.id
SET cr.garden_id = m.garden_id`
	return db.Exec(reminderFill).Error
}

func seed(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	logger := slog.Default()

	adminHash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	userHash, _ := bcrypt.GenerateFromPassword([]byte("user123"), bcrypt.DefaultCost)
	admin := &model.User{Username: "admin", Email: "admin@gbplantwiki.local", PasswordHash: string(adminHash), Nickname: "园艺管理员", Role: "admin"}
	user := &model.User{Username: "gardener", Email: "gardener@gbplantwiki.local", PasswordHash: string(userHash), Nickname: "绿手指", Role: "user"}
	if err := db.Create(admin).Error; err != nil {
		return err
	}
	if err := db.Create(user).Error; err != nil {
		return err
	}

	plants := []model.PlantSpecies{
		{Family: "天南星科", Genus: "龟背竹属", Name: "龟背竹", Alias: "蓬莱蕉", Type: constants.PlantTypeFoliage, Origin: "墨西哥", TempMin: 18, TempMax: 30, LightRequirement: "散射光", WaterFrequency: "每周2次", Description: "耐阴的大型观叶植物，叶片具独特裂孔。", ImageURLs: `["https://images.unsplash.com/photo-1524594152303-9fd13543fe6e?w=600"]`},
		{Family: "景天科", Genus: "拟石莲属", Name: "多肉吉娃娃", Alias: "吉娃娃", Type: constants.PlantTypeSucculent, Origin: "墨西哥", TempMin: 10, TempMax: 28, LightRequirement: "充足直射光", WaterFrequency: "每月2次", Description: "叶片莲座状排列，日照充足时叶尖泛红。", ImageURLs: `["https://images.unsplash.com/photo-1509423350716-97f9360b4e09?w=600"]`},
		{Family: "睡莲科", Genus: "睡莲属", Name: "碗莲", Alias: "微型荷花", Type: constants.PlantTypeAquatic, Origin: "中国", TempMin: 15, TempMax: 35, LightRequirement: "全日照", WaterFrequency: "保持水位", Description: "小型水生花卉，适合庭院水缸栽培。", ImageURLs: `["https://images.unsplash.com/photo-1508766917616-d22f3f1eea14?w=600"]`},
		{Family: "蔷薇科", Genus: "月季属", Name: "月季", Alias: "月月红", Type: constants.PlantTypeFlower, Origin: "中国", TempMin: 5, TempMax: 30, LightRequirement: "全日照", WaterFrequency: "每周3次", Description: "花型丰富、花期长的经典观赏花卉。", ImageURLs: `["https://images.unsplash.com/photo-1496062031456-07b8f162a322?w=600"]`},
		{Family: "百合科", Genus: "芦荟属", Name: "库拉索芦荟", Alias: "真芦荟", Type: constants.PlantTypeSucculent, Origin: "非洲", TempMin: 10, TempMax: 32, LightRequirement: "明亮散射光", WaterFrequency: "每两周1次", Description: "多年生常绿多肉植物，具有美容护肤价值。", ImageURLs: `["https://images.unsplash.com/photo-1512418418704-1e26dc9c8a7a?w=600"]`},
		{Family: "柏科", Genus: "圆柏属", Name: "清香木", Alias: "细叶清香木", Type: constants.PlantTypeFoliage, Origin: "中国西南", TempMin: 8, TempMax: 30, LightRequirement: "半日照", WaterFrequency: "每周1次", Description: "常绿灌木，叶片揉碎有清香，适合盆栽。", ImageURLs: `["https://images.unsplash.com/photo-1463320726281-696a485928c7?w=600"]`},
	}
	if err := db.Create(&plants).Error; err != nil {
		return err
	}

	articles := []model.CareArticle{
		{UserID: admin.ID, Title: "春季换盆全攻略：时机、方法与注意事项", Content: "春季气温回升后是换盆的最佳时机。换盆前停止浇水3天，小心脱盆，修剪烂根并消毒，选择比原盆大1-2号的透气花盆，底部垫陶粒排水层……", Cover: "https://images.unsplash.com/photo-1459156212016-c812468e2115?w=800", TopicTag: constants.TopicTagRepotting, Status: constants.ArticleStatusPublished, ViewCount: 128},
		{UserID: admin.ID, Title: "多肉植物施肥要点：薄肥勤施", Content: "多肉施肥宜稀薄，生长季每月一次稀释液肥即可，休眠期停止施肥，避免肥害烧根……", Cover: "https://images.unsplash.com/photo-1485955900006-10f4d324d411?w=800", TopicTag: constants.TopicTagFertilizing, Status: constants.ArticleStatusPublished, ViewCount: 96},
		{UserID: user.ID, Title: "月季夏季修剪与控旺", Content: "月季夏季修剪以轻剪为主，剪除残花和细弱枝，保留健壮枝条促进复花……", Cover: "https://images.unsplash.com/photo-1496062031456-07b8f162a322?w=800", TopicTag: constants.TopicTagPruning, Status: constants.ArticleStatusPublished, ViewCount: 210},
		{UserID: admin.ID, Title: "常见介壳虫的识别与防治", Content: "介壳虫常附着在叶背和枝干，可用酒精棉擦拭，严重时喷洒矿物油乳剂……", Cover: "https://images.unsplash.com/photo-1530836369250-ef72a3f5cda8?w=800", TopicTag: constants.TopicTagPestControl, Status: constants.ArticleStatusPublished, ViewCount: 154},
		{UserID: user.ID, Title: "龟背竹扦插繁殖实操", Content: "选取带气生根的健壮枝条，切口晾干后插入湿润的蛭石中，保持湿度约三周生根……", Cover: "https://images.unsplash.com/photo-1524594152303-9fd13543fe6e?w=800", TopicTag: constants.TopicTagPropagation, Status: constants.ArticleStatusPublished, ViewCount: 67},
	}
	if err := db.Create(&articles).Error; err != nil {
		return err
	}

	pests := []model.DiseasePest{
		{PlantSpeciesID: plants[3].ID, Name: "月季黑斑病", Symptoms: "叶片出现黑色圆形斑点，边缘呈放射状，严重时叶片脱落。", Cause: "高温高湿、通风不良，病原为蔷薇黑斑菌。", Treatment: "及时摘除病叶，喷施代森锰锌或苯醚甲环唑，每周一次连续2-3次。", RecommendedMedicine: "代森锰锌、苯醚甲环唑", Keywords: "黑斑,黄叶,月季", Images: `[]`},
		{PlantSpeciesID: plants[1].ID, Name: "多肉介壳虫", Symptoms: "叶腋处出现白色棉絮状物，叶片发黏发黄。", Cause: "通风差、湿度大，虫源为蚧壳虫若虫。", Treatment: "人工刮除后用酒精擦拭，严重时喷施噻嗪酮。", RecommendedMedicine: "噻嗪酮、矿物油乳剂", Keywords: "介壳虫,白色,黏", Images: `[]`},
		{PlantSpeciesID: plants[0].ID, Name: "龟背竹叶斑病", Symptoms: "叶片出现褐色水渍状病斑，逐渐扩大干枯。", Cause: "浇水过多、长期积水，病原真菌感染。", Treatment: "控水通风，剪除病叶，喷施多菌灵。", RecommendedMedicine: "多菌灵", Keywords: "叶斑,烂叶", Images: `[]`},
		{PlantSpeciesID: 0, Name: "红蜘蛛", Symptoms: "叶面出现细密黄白色斑点，叶背有蛛网。", Cause: "空气干燥、高温，螨虫滋生。", Treatment: "增加湿度，喷施阿维菌素或哒螨灵。", RecommendedMedicine: "阿维菌素、哒螨灵", Keywords: "红蜘蛛,螨,黄点", Images: `[]`},
	}
	if err := db.Create(&pests).Error; err != nil {
		return err
	}

	// Two independently registered pots for the demo user: each pot has its
	// own pot number, location and takeover date.
	pots := []model.UserGarden{
		{UserID: user.ID, PlantSpeciesID: plants[3].ID, PotNo: "P0001", Nickname: "阳台月季", OwnedSince: daysAgo(120), Location: "南阳台", Status: model.PotActive},
		{UserID: user.ID, PlantSpeciesID: plants[0].ID, PotNo: "P0002", Nickname: "客厅龟背竹", OwnedSince: daysAgo(200), Location: "客厅窗边", Status: model.PotActive},
	}
	if err := db.Create(&pots).Error; err != nil {
		return err
	}

	reminders := []model.CareReminder{
		{UserID: user.ID, GardenID: pots[0].ID, PlantSpeciesID: plants[3].ID, TaskTitle: "给月季补充缓释肥", RemindDate: daysFromNow(3), Frequency: "monthly", Status: model.ReminderPending},
		{UserID: user.ID, GardenID: pots[1].ID, PlantSpeciesID: plants[0].ID, TaskTitle: "龟背竹叶片擦拭除尘", RemindDate: daysFromNow(1), Frequency: "weekly", Status: model.ReminderPending},
	}
	if err := db.Create(&reminders).Error; err != nil {
		return err
	}

	questions := []model.Question{
		{UserID: user.ID, Title: "新买的月季叶子发黄怎么办？", Content: "刚上盆一周，叶片边缘发黄，是不是浇水太多？", Images: `[]`, Status: "open"},
		{UserID: user.ID, Title: "多肉徒长了如何补救？", Content: "冬季光照不足，多肉长高了，可以砍头吗？", Images: `[]`, Status: "open"},
	}
	if err := db.Create(&questions).Error; err != nil {
		return err
	}

	answers := []model.Answer{
		{QuestionID: questions[0].ID, UserID: admin.ID, Content: "新上盆植物根系未恢复，建议先放在散射光处缓苗，见干见湿浇水，避免积水。", LikeCount: 5},
		{QuestionID: questions[1].ID, UserID: admin.ID, Content: "可以砍头繁殖，砍下的头部晾干后重新扦插，母株会萌发侧芽。", LikeCount: 8},
	}
	if err := db.Create(&answers).Error; err != nil {
		return err
	}

	logger.Info("gbplantwiki seed data created",
		"users", 2, "plants", len(plants), "articles", len(articles),
		"pests", len(pests), "pots", len(pots), "reminders", len(reminders),
		"questions", len(questions), "answers", len(answers))
	return nil
}
