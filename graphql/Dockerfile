FROM node:alpine

# Install PostGraphile and PostGraphile connection filter plugin
RUN npm install -g postgraphile
RUN npm install -g postgraphile-plugin-connection-filter

#RENAME JSON FUNCS TO JSONB
RUN   find /usr/local/lib/node_modules/postgraphile/node_modules/graphile-build-pg/node8plus/ -type f -name "*.js*"
RUN   find /usr/local/lib/node_modules/postgraphile/node_modules/graphile-build-pg/node8plus/ -type f -name "*.js*" -print0 | xargs -0 sed -i '' -e 's/to_json/to_jsonb/g'
RUN   find /usr/local/lib/node_modules/postgraphile/node_modules/graphile-build-pg/node8plus/ -type f -name "*.js*" -print0 | xargs -0 sed -i '' -e 's/json_agg/jsonb_agg/g'
RUN   find /usr/local/lib/node_modules/postgraphile/node_modules/graphile-build-pg/node8plus/ -type f -name "*.js*" -print0 | xargs -0 sed -i '' -e 's/json_build_object/jsonb_build_object/g'


COPY postgraphile.schema /postgraphile.schema

EXPOSE 5000
ENTRYPOINT ["postgraphile", "-n", "0.0.0.0"]