FROM node:alpine

# Install PostGraphile and PostGraphile connection filter plugin
RUN npm install -g postgraphile
RUN npm install -g postgraphile-plugin-connection-filter

RUN   sed -i 's/to_json/to_jsonb/g' /usr/local/lib/node/postgraphile/*
RUN   sed -i 's/json_agg/jsonb_agg/g' /usr/local/lib/node/postgraphile/*
RUN   sed -i 's/json_build_object/jsonb_build_object/g' /usr/local/lib/node/postgraphile/*

COPY postgraphile.schema /postgraphile.schema

EXPOSE 5000
ENTRYPOINT ["postgraphile", "-n", "0.0.0.0"]