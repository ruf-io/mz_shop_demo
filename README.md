# Real-Time E-Commerce Demo

This app demonstrates the real-time incremental computation capabilities of Materialize in an e-commerce website.

**An e-commerce business wants to understand:**

- **Order trends** throughout the day to discern patterns.
- What is selling the most?
  - Understand supply/demand and manage inventory.
  - Show inventory status in the website to encourage users to buy.
- **Conversion Funnel:** Effectiveness of the website in converting pageviews to actual buys.
- **Low-stock Alerts:** Generate alerts and automatically place orders to the warehouse if a specific item is close to running out of stock

We'll build materialized views that answer most of the questions by providing data in a business intelligence dashboard, and we'll pipe data out to an API to provide answers to the other questions.

To generate the data we'll simulate **users**, **items**, **purchases** and **pageviews** on a fictional e-commerce website.

To simplify deploying all of this infrastructure, the demo is enclosed in a series of Docker images glued together via Docker Compose. As a secondary benefit, you can run the demo via Linux, an EC2 VM instance, or a Mac laptop.

## What to Expect

Our load generator (`loadgen`) is a [python script](loadgen/generate_load.py) that does two things:

1. It seeds MySQL with `item`, `user` and `purchase` tables, and then begins rapidly adding `purchase` rows that join an item and a user. _(~20 per second)_
2. It simultaneously begins sending JSON-encoded `pageview` events directly to kafka. _(~1,000 per second)_

As the database writes occur, Debezium/Kafka stream the changes out of MySQL. Materialize subscribes to this change feed and maintains our materialized views with the incoming data––materialized views typically being some report whose information we're regularly interested in viewing.

For example, if we wanted real time statistics of total pageviews and orders by item, Materialize could maintain that report as a materialized view. And, in fact,
that is exactly what this demo will show.

## Prepping Mac Laptops

If you're on a Mac laptop, you might want to increase the amount of memory
available to Docker Engine.

1. From the Docker Desktop menu bar app, select **Preferences**.
1. Go to the **Advanced** tab.
1. Select at least **8 GiB** of **Memory**.
1. Click **Apply and Restart**.

## Running the Demo

1. Bring up the Docker Compose containers in the background:

    ```shell session
    $ docker-compose up -d
    Creating network "demo_default" with the default driver
    Creating demo_chbench_1      ... done
    Creating demo_mysql_1        ... done
    Creating demo_materialized_1 ... done
    Creating demo_connector_1    ... done
    Creating demo_zookeeper_1    ... done
    Creating demo_kafka_1        ... done
    Creating demo_connect_1         ... done
    Creating demo_schema-registry_1 ... done
    ```

    If all goes well, you'll have MySQL, ZooKeeper, Kafka, Kafka Connect,
    Materialize, and a load generator running, each in their own container, with
    Debezium configured to ship changes from MySQL into Kafka.

2. Launch the Materialize CLI.

    ```shell session
    psql -U materialize -h localhost -p 6875 materialize
    ```

3. Now that you're in the Materialize CLI (denoted by the terminal prefix
   `mz>`), define all of the tables in `mysql.shop` as Kafka sources in
   Materialize.

    ```sql
    CREATE SOURCE purchases
    FROM KAFKA BROKER 'kafka:9092' TOPIC 'mysql.shop.purchases'
    FORMAT AVRO USING CONFLUENT SCHEMA REGISTRY 'http://schema-registry:8081'
    ENVELOPE DEBEZIUM;

    CREATE SOURCE items
    FROM KAFKA BROKER 'kafka:9092' TOPIC 'mysql.shop.items'
    FORMAT AVRO USING CONFLUENT SCHEMA REGISTRY 'http://schema-registry:8081'
    ENVELOPE DEBEZIUM;

    CREATE SOURCE users
    FROM KAFKA BROKER 'kafka:9092' TOPIC 'mysql.shop.users'
    FORMAT AVRO USING CONFLUENT SCHEMA REGISTRY 'http://schema-registry:8081'
    ENVELOPE DEBEZIUM;
    ```

    Because these sources are pulling message schema data from the registry, materialize automatically knows the column types to use for each attribute.

4. We'll also want to create a JSON-formatted source for the pageviews:

    ```sql
    CREATE SOURCE json_pageviews
    FROM KAFKA BROKER 'kafka:9092' TOPIC 'pageviews'
    FORMAT BYTES;
    ```

    With JSON-formatted messages, we don't know the schema so the JSON is pulled in as raw bytes and we still need to CAST data into the proper columns and types.

5. Next we will create our first Materialized View, summarizing pageviews by item:

    ```sql
    CREATE MATERIALIZED VIEW item_pageviews AS
        SELECT
        (regexp_match((data->'url')::STRING, '/products/(\d+)')[1])::INT AS item_id,
        COUNT(*) as pageviews
        FROM (
        SELECT CAST(data AS jsonb) AS data
        FROM (
            SELECT convert_from(data, 'utf8') AS data
            FROM json_pageviews
        )) GROUP BY 1;
    ```

    As you can see here, we are doing a couple extra steps to get the pageview data into the format we need:

    1. We are converting from raw bytes to utf8 encoded text:

       ```sql
       SELECT convert_from(data, 'utf8') AS data
            FROM json_pageviews
        ```

    2. We are using postgres JSON notation (`data->'url'`), type casts (`::STRING`) and regexp_match function to extract only the item_id from the raw pageview URL.

       ```sql
       (regexp_match((data->'url')::STRING, '/products/(\d+)')[1])::INT AS item_id,
       ```

6. Now if you select results from the view, you should see data populating:

    ```sql
    SELECT * FROM item_pageviews ORDER BY pageviews DESC LIMIT 10;
    ```

    If you re-run it a few times you should see the pageview counts changing as new data comes in and is materialized in realtime.

7. Let's create some more materialized views:

    Purchase Summary:

    ```sql
    CREATE MATERIALIZED VIEW purchase_summary AS 
        SELECT
            item_id,
            SUM(purchase_price) as revenue,
            COUNT(id) AS orders,
            SUM(quantity) AS items_sold
        FROM purchases GROUP BY 1;
    ```

    Item Summary: _(Using purchase summary and pageview summary internally)_

    ```sql
    CREATE MATERIALIZED VIEW item_summary AS
        SELECT
            items.name,
            purchase_summary.items_sold,
            purchase_summary.orders,
            purchase_summary.revenue,
            item_pageviews.pageviews,
            CASE WHEN item_pageviews.pageviews IS NULL THEN 0.0 ELSE purchase_summary.orders / item_pageviews.pageviews::FLOAT END AS conversion_rate
        FROM items
        JOIN purchase_summary ON purchase_summary.item_id = items.id
        JOIN item_pageviews ON item_pageviews.item_id = items.id;
    ```

    This last one shows some of the advanced JOIN capabilities of Materialize, we're joining our two previous views with items to create a complex roll-up summary of purchases, pageviews, and conversion rates.

    If you select from `item_summary` you can see the results in real-time:

    ```sql
    SELECT * FROM item_summary ORDER BY conversion_rate DESC LIMIT 10;
    ```

8. Close out of the Materialize CLI (<kbd>Ctrl</kbd> + <kbd>D</kbd>).

9. Watch the report change using the `watch-sql` container, which continually
   streams changes from Materialize to your terminal.

    ```shell
    ./mzcompose run cli watch-sql "SELECT * FROM purchase_sum_by_region"
    ```

11. Once you're sufficiently wowed, close out of the `watch-sql` container
   (<kbd>Ctrl</kbd> + <kbd>D</kbd>), and bring the entire demo down.

    ```shell
    ./mzcompose down
    ```
