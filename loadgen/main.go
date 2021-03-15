package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"time"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

const (
	userSeedCount      = 1000
	itemSeedCount      = 200
	purchaseSeedCount  = 100
	purchaseGenCount   = 10000
	purchaseGenEveryMS = 100
	item_inventory_min = 10
	item_inventory_max = 1000
	first_names = [20]string{"Liam", "Olivia", "Noah", "Emma", "Oliver", "Ava", "William", "Sophia","Elijah", "Isabella", "James", "Charlotte", "Benjamin", "Amelia", "Lucas", "Mia", "Mason", "Harper", "Ethan", "Evelyn"}
	last_names = [20]string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson", "Thomas", "Taylor", "Moore", "Jackson", "Martin"}
	descriptors = [20]string{ "Adaptable", "Ambitious", "Brave", "Calm", "Cheerful", "Classic", "Cultured", "Delightful", "Delicate", "Familiar", "Fearless", "Gentle", "Harmonious", "Joyous", "Lovely", "Lucky", "Noble", "Original", "Timeless", "Wise" }
	products = [15]string{"Fedora", "Boater", "Snapback", "Trilby", "Panama", "Bowler", "Dad", "Newsboy", "Flat Cap", "Beanie", "Bucket", "Baseball", "Trapper", "Pork Pie", "Top Hat"}
)

func doExec(db *sql.DB, str string) {
	_, err := db.Exec(str)
	if err != nil {
		panic(err.Error())
	}
}

func genPrepare(db *sql.DB, str string) *sql.Stmt {
	prep, err := db.Prepare(str)
	if err != nil {
		panic(err.Error())
	}

	return prep
}

func main() {
	db, err := sql.Open("mysql", "root:debezium@tcp(mysql:3306)/")
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	doExec(db, "DROP DATABASE shop;")

	doExec(db, "CREATE DATABASE shop;")

	doExec(db, `CREATE TABLE  shop.user
		(
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			email VARCHAR(255),
			is_vip BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		);`)

	doExec(db, `CREATE TABLE shop.purchase
		(
			id SERIAL PRIMARY KEY,
			user_id BIGINT UNSIGNED REFERENCES user(id),
			status TINYINT UNSIGNED DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		);`)
	
	doExec(db, `CREATE TABLE shop.purchase_item
		(
			id SERIAL PRIMARY KEY,
			purchase_id BIGINT UNSIGNED REFERENCES purchase(id),
			item_id BIGINT UNSIGNED REFERENCES item(id),
			quantity INT UNSIGNED DEFAULT 1,
			purchase_price DECIMAL(12,2),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`)

	doExec(db, `CREATE TABLE shop.item
		(
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			price DECIMAL(7,2),
			daily_inventory INT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		);`)

	insertItem := genPrepare(db,
		"INSERT INTO shop.item (name, price, daily_inventory) VALUES ( ?, ?, ? );")
	insertUser := genPrepare(db,
		"INSERT INTO shop.user (name, email, is_vip) VALUES ( ?, ?, ? );")
	insertPurchase := genPrepare(db,
		"INSERT INTO shop.purchase (user_id) VALUES ( ? );")
	insertPurchaseItem := genPrepare(db,
		"INSERT INTO shop.purchase_item (purchase_id, item_id, quantity, purchase_price) VALUES ( ?, ?, ?, ? );")

	rndSrc := rand.NewSource(time.Now().UnixNano())
	rnd := rand.New(rndSrc)

	
	fmt.Println("Seeding %d shop items...", itemSeedCount)
	for i := 0; i < itemSeedCount; i++ {
		// Insert user with random region.
		_, err = insertItem.Exec(
			"The " + descriptors[rnd.Intn(len(descriptors))] + " " + products[rnd.Intn(len(products))],
			rnd.Intn(item_price_max - item_price_min) + item_price_min,
			rnd.Intn(item_inventory_max - item_inventory_min) + item_inventory_min
		)

		if err != nil {
			panic(err.Error())
		}
	}

	fmt.Println("Seeding %d users...", userSeedCount)
	for i := 0; i < userSeedCount; i++ {
		// Insert user with random name/email/is_vip status.
		var user_name = first_names[rnd.Intn(len(first_names))] + " " + last_names[rnd.Intn(len(last_names))]
		var is_vip = false
		if rnd.Intn(10) == 9 {
			is_vip = true
		}
		_, err = insertUser.Exec(
			user_name,
			strings.Replace(user_name, " ", ".") + "@gmail.com",
			is_vip
		)

		if err != nil {
			panic(err.Error())
		}
	}

	fmt.Println("Getting shop items...")
	items = [][]
	rows, err := db.Query("SELECT id, price FROM items;")
	if err != nil {
		// handle this error better than this
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var price float64
		err = rows.Scan(&id, &price)
		if err != nil {
		// handle this error
		panic(err)
		}
		items = append(items, [id, price])
	}
	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		panic(err)
	}

	fmt.Println("Creating purchases...")
	for i := 0; i < purchaseSeedCount; i++ {
		// Insert purchase with random amount and user.
		_, err = insertPurchase.Exec(rnd.Float64()*10000, rnd.Intn(userIDCount)+userIDMin)
		if err != nil {
			panic(err.Error())
		}
	}

	// Do some math to let users understand how fast data changes and how long
	// the loadgen is set to run.
	sleepTime := purchaseGenEveryMS * time.Millisecond
	purchaseGenPerSecond := time.Second / sleepTime
	purchaseGenPeriod := purchaseGenCount / (purchaseGenPerSecond * 60)

	fmt.Printf("Generating %d purchases (%d/s for %dm)\n",
		purchaseGenCount, purchaseGenPerSecond, purchaseGenPeriod)

	// Continue generating purchases.
	for i := 0; i < 10000; i++ {
		_, err = insertPurchase.Exec(rnd.Float64()*10000, rnd.Intn(userIDCount)+userIDMin)

		if err != nil {
			panic(err.Error())
		}

		// Materialize requires some activity on row after source is created to
		// ingest it. This frequency is overkill but was the simplest way to
		// ensure some activity occurs at some indeterminate point after the
		// loadgen starts.
		timeNow := time.Now()
		updateRegionTime.Exec(timeNow)
		updateUserTime.Exec(timeNow)
		time.Sleep(sleepTime)
	}

	fmt.Println("Done generating purchases. ttfn.")
}
