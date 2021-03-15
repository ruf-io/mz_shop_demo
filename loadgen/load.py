from mysql.connector import connect, Error
import barnum, random

# CONFIG
userSeedCount      = 1000
itemSeedCount      = 200
purchaseSeedCount  = 100
purchaseGenCount   = 10000
purchaseGenEveryMS = 100
itemInventoryMin   = 10
itemInventoryMax   = 1000
itemPriceMin       = 5
itemPriceMax       = 500
kafkaTopic         = "pageview"

item_insert = "INSERT INTO shop.items (name, price, daily_inventory) VALUES ( ?, ?, ? )"
user_insert = "INSERT INTO shop.users (email, is_vip) VALUES ( ?, ? )"
purchase_insert = "INSERT INTO shop.purchases (user_id, item_id, quantity, purchase_price) VALUES ( ?, ?, ?, ? )"

try:
    with connect(
        host="localhost",
        user='root',
        password='debezium',
    ) as connection:
        with connection.cursor() as cursor:
            print("Initializing shop database...")
            cursor.execute('DROP DATABASE IF EXISTS shop;')
            cursor.execute('CREATE DATABASE shop;')
            cursor.execute(
                """CREATE TABLE  shop.users
                    (
                        id SERIAL PRIMARY KEY,
                        email VARCHAR(255),
                        is_vip BOOLEAN DEFAULT FALSE,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
                );"""
            )
            cursor.execute(
                """CREATE TABLE shop.items
                    (
                        id SERIAL PRIMARY KEY,
                        name VARCHAR(100),
                        price DECIMAL(7,2),
                        daily_inventory INT,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
                );"""
            )
            cursor.execute(
                """CREATE TABLE shop.purchases
                    (
                        id SERIAL PRIMARY KEY,
                        user_id BIGINT UNSIGNED REFERENCES user(id),
                        item_id BIGINT UNSIGNED REFERENCES item(id),
                        status TINYINT UNSIGNED DEFAULT 1,
                        quantity INT UNSIGNED DEFAULT 1,
                        purchase_price DECIMAL(12,2),
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
                );"""
            )
            connection.commit()
            print("Seeding data...")
            cursor.executemany(
                item_insert,
                [
                    [
                        barnum.create_nouns(),
                        random.randint(itemPriceMin*100,itemPriceMax*100)/100,
                        random.randint(itemInventoryMin,itemInventoryMax)
                    ] for i in range(itemSeedCount)
                ]
            )
            cursor.executemany(
                user_insert,
                [
                    [
                        barnum.create_email(),
                        (random.randint(0,10) > 8)
                    ] for i in range(userSeedCount)
                ]
            )
            connection.commit()
except Error as e:
    print(e)