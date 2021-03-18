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

The [docker-compose file](docker-compose.yml) spins up 9 containers with the following names, connections and roles:
![Shop demo infra](https://user-images.githubusercontent.com/11527560/111649810-18e23e80-87db-11eb-96c0-6518ef7b87ba.png)

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
    Creating demo_zookeeper_1    ... done
    Creating demo_connector_1       ... done
    Creating demo_materialized_1    ... done
    Creating demo_mysql_1           ... done
    Creating demo_kafka_1        ... done
    Creating demo_schema-registry_1 ... done
    Creating demo_connect_1         ... done
    Creating demo_loadgen_1         ... done
    Creating demo_graphql_1         ... done
    Creating demo_metabase_1        ... done
    ```

    If all goes well, you'll have everything running in their own containers, with Debezium configured to ship changes from MySQL into Kafka.

2. Launch the Materialize CLI.

    ```shell session
    psql -U materialize -h localhost -p 6875 materialize
    ```

3. Now that you're in the Materialize CLI, define all of the tables in `mysql.shop` as Kafka sources:

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

    With JSON-formatted messages, we don't know the schema so the JSON is pulled in as raw bytes and we still need to CAST data into the proper columns and types. We'll show that in the step below.

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

    If you re-run it a few times you should see the pageview counts changing as new data comes in and gets materialized in real time.

7. Let's create some more materialized views:

    **Purchase Summary:**

    ```sql
    CREATE MATERIALIZED VIEW purchase_summary AS 
        SELECT
            item_id,
            SUM(purchase_price) as revenue,
            COUNT(id) AS orders,
            SUM(quantity) AS items_sold
        FROM purchases GROUP BY 1;
    ```

    **Item Summary:** _(Using purchase summary and pageview summary internally)_

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

    This view shows some of the JOIN capabilities of Materialize, we're joining our two previous views with items to create a summary of purchases, pageviews, and conversion rates.

    If you select from `item_summary` you can see the results in real-time:

    ```sql
    SELECT * FROM item_summary ORDER BY conversion_rate DESC LIMIT 10;
    ```

    **Remaining Stock:**

    ```sql
    CREATE MATERIALIZED VIEW remaining_stock AS
        SELECT
          items.id as item_id,
          MAX(items.inventory) - SUM(purchases.quantity) AS remaining_stock,
          1 as trend_rank
        FROM items
        JOIN purchases ON purchases.item_id = items.id AND purchases.created_at > items.inventory_updated_at
        GROUP BY items.id;
    ```

    **Trending Items:**

    Here we are doing a bit of a hack because Materialize doesn't yet support window functions like `RANK`. So instead we are doing a self join on `purchase_summary` and counting up the items with _more purchases than the current item_ to get a basic "trending" rank datapoint.

    ```sql
    CREATE MATERIALIZED VIEW trending_items AS
        SELECT
            p1.item_id,
            SUM(CASE WHEN p2.items_sold > p1.items_sold THEN 1 ELSE 0 END) as trend_rank
        FROM purchase_summary p1
        FULL JOIN purchase_summary p2 ON 1 = 1
        GROUP BY 1;
    ```

    Lastly, let's bring the trending items and remaining stock views together to expose to our graphql API:

    ```sql
    CREATE MATERIALIZED VIEW item_metadata AS
        SELECT
            rs.item_id as id, rs.remaining_stock, ti.trend_rank
        FROM remaining_stock rs
        JOIN trending_items ti ON ti.item_id = rs.item_id;
    ```

8. Now you've materialized some views that we can use in a business intelligence tool, metabase, and in a graphQL API (postgraphile.) Close out of the Materialize CLI (<kbd>Ctrl</kbd> + <kbd>D</kbd>).

9. **Run Metabase**

    1. In a browser, go to <localhost:3030>.

    2. Click **Let's get started**.

    3. Complete the first set of fields asking for your email address. This
       information isn't crucial for anything but does have to be filled in.

    4. On the **Add your data** page, fill in the following information:

        Field             | Enter...
        ----------------- | ----------------
        Database          | **Materialize**
        Name              | **tpcch**
        Host              | **materialized**
        Port              | **6875**
        Database name     | **materialize**
        Database username | **materialize**
        Database password | Leave empty.

    5. Proceed past the screens until you reach your primary dashboard.

## Create dashboards

    1. Click **Ask a question**.

    2. Click **Native query**.

    3. From **Select a database**, select **tpcch**.

    4. In the query editor, enter:

        ```sql
        SELECT * FROM query01;
        ```

10. Once you're sufficiently wowed, close out of the `watch-sql` container
   (<kbd>Ctrl</kbd> + <kbd>D</kbd>), and bring the entire demo down.

    ```shell
    ./mzcompose down
    ```
