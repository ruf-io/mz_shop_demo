package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"time"
	"strings"
	"github.com/Shopify/sarama"
	
	_ "github.com/go-sql-driver/mysql"
)


type item struct {
    id    int
    price float64
}

const (
	userSeedCount      = 1000
	itemSeedCount      = 200
	purchaseSeedCount  = 100
	purchaseGenCount   = 10000
	purchaseGenEveryMS = 100
	itemInventoryMin   = 10
	itemInventoryMax   = 1000
	itemPriceMin       = 5.0
	itemPriceMax       = 500.0
	kafkaTopic         = "pageview"
)

var (
	items []item
	kafkaBrokers = []string{"127.0.0.1:9092"}
	firstNames   = [20]string{"Liam", "Olivia", "Noah", "Emma", "Oliver", "Ava", "William", "Sophia","Elijah", "Isabella", "James", "Charlotte", "Benjamin", "Amelia", "Lucas", "Mia", "Mason", "Harper", "Ethan", "Evelyn"}
	lastNames    = [20]string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson", "Thomas", "Taylor", "Moore", "Jackson", "Martin"}
	descriptors  = [20]string{ "Adaptable", "Ambitious", "Brave", "Calm", "Cheerful", "Classic", "Cultured", "Delightful", "Delicate", "Familiar", "Fearless", "Gentle", "Harmonious", "Joyous", "Lovely", "Lucky", "Noble", "Original", "Timeless", "Wise" }
	products     = [15]string{"Fedora", "Boater", "Snapback", "Trilby", "Panama", "Bowler", "Dad", "Newsboy", "Flat Cap", "Beanie", "Bucket", "Baseball", "Trapper", "Pork Pie", "Top Hat"}
)

func newProducer() (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Partitioner = sarama.NewRandomPartitioner
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer(kafkaBrokers, config)

	return producer, err
}

func prepareMessage(topic, message string) *sarama.ProducerMessage {
	msg := &sarama.ProducerMessage{
		Topic:     topic,
		Partition: -1,
		Value:     sarama.StringEncoder(message),
	}

	return msg
}

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
	db, err := sql.Open("mysql", "root:debezium@tcp(127.0.0.1:3306)/")
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	doExec(db, "DROP DATABASE IF EXISTS shop;")

	doExec(db, "CREATE DATABASE shop;")

	doExec(db, `CREATE TABLE  shop.users
		(
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			email VARCHAR(255),
			is_vip BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		);`)

	doExec(db, `CREATE TABLE shop.items
		(
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			price DECIMAL(7,2),
			daily_inventory INT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		);`)

	doExec(db, `CREATE TABLE shop.purchases
		(
			id SERIAL PRIMARY KEY,
			user_id BIGINT UNSIGNED REFERENCES user(id),
			item_id BIGINT UNSIGNED REFERENCES item(id),
			status TINYINT UNSIGNED DEFAULT 1,
			quantity INT UNSIGNED DEFAULT 1,
			purchase_price DECIMAL(12,2),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		);`)

	insertItem := genPrepare(db,
		"INSERT INTO shop.items (name, price, daily_inventory) VALUES ( ?, ?, ? );")
	insertUser := genPrepare(db,
		"INSERT INTO shop.users (name, email, is_vip) VALUES ( ?, ?, ? );")
	insertPurchase := genPrepare(db,
		"INSERT INTO shop.purchases (user_id, item_id, quantity, purchase_price) VALUES ( ?, ?, ?, ? );")
	
	rndSrc := rand.NewSource(time.Now().UnixNano())
	rnd := rand.New(rndSrc)

	
	fmt.Printf("Seeding %d shop items...", itemSeedCount)
	for i := 0; i < itemSeedCount; i++ {
		// Insert user with random region.
		_, err = insertItem.Exec(
			fmt.Sprintf("The %s %s", descriptors[rnd.Intn(len(descriptors))], products[rnd.Intn(len(products))]),
			rnd.Intn(itemPriceMax - itemPriceMin) + itemPriceMin,
			(rnd.Intn(itemInventoryMax - itemInventoryMin) + itemInventoryMin),
		)

		if err != nil {
			panic(err.Error())
		}
	}

	fmt.Printf("Seeding %d users...", userSeedCount)
	for i := 0; i < userSeedCount; i++ {
		// Insert user with random name/email/is_vip status.
		var user_name = fmt.Sprintf("%s %s", firstNames[rnd.Intn(len(firstNames))], lastNames[rnd.Intn(len(lastNames))])
		var email = fmt.Sprintf("%s@%s", strings.Replace(user_name, " ", ".", -1), "gmail.com")
		var is_vip = false
		if rnd.Intn(10) == 9 {
			is_vip = true
		}
		_, err = insertUser.Exec(
			user_name,
			email,
			is_vip,
		)

		if err != nil {
			panic(err.Error())
		}
	}

	fmt.Println("Getting shop items...")
	rows, err := db.Query("SELECT id, price FROM shop.items;")
	if err != nil {
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
		items = append(items, item{id, price})
	}
	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Loaded %d items", len(items))


	// Do some math to let users understand how fast data changes and how long
	// the loadgen is set to run.
	sleepTime := purchaseGenEveryMS * time.Millisecond
	purchaseGenPerSecond := time.Second / sleepTime
	purchaseGenPeriod := purchaseGenCount / (purchaseGenPerSecond * 60)

	fmt.Printf("Generating %d purchases (%d/s for %dm)\n",
		purchaseGenCount, purchaseGenPerSecond, purchaseGenPeriod)

	//Initialize kafka producer
	producer, err := newProducer()
	if err != nil {
		fmt.Println("Could not create producer: ", err)
	}

	// Continue generating purchases.
	for i := 0; i < 10000; i++ {

		var purchase_item = items[rnd.Intn(len(items))]
		var quantity = rnd.Intn(4) + 1
		var purchase_user = rnd.Intn(userSeedCount)

		//WRITE PURCHASE PAGEVIEW
		msg := prepareMessage(kafkaTopic, fmt.Sprintf("{'user_id': %d, 'item_id': %d, 'received_at': %d}", purchase_user, purchase_item, time.Now().Unix()))
		partition, offset, err := producer.SendMessage(msg)
		if err != nil {
			fmt.Printf("%s error occured.", err.Error())
		}
		
		//WRITE SOME OTHER RANDOM PAGEVIEWS
		for j := 0; j < 10; j++ {
			msg := prepareMessage(kafkaTopic, fmt.Sprintf("{'user_id': %d, 'item_id': %d, 'received_at': %d}", rnd.Intn(userSeedCount), rnd.Intn(itemSeedCount), time.Now().Unix()))
			partition, offset, err := producer.SendMessage(msg)
			if err != nil {
				fmt.Printf("%s error occured.", err.Error())
			}
		}
		
		time.Sleep(sleepTime)

		//WRITE PURCHASE
		
		_, err = insertPurchase.Exec(
			purchase_user,
			purchase_item.id,
			quantity,
			(purchase_item.price * float64(quantity)),
		)

		if err != nil {
			panic(err.Error())
		}
	}

	fmt.Println("Done generating purchases. ttfn.")
}
