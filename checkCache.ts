/**
 * @Function checkCache
 *
 * Takes in a cache key (string), a numeric time to save the response to the cache, and a callback function to get the response that is to be in the cache, and first checks the cache for the key and returns the cache response if the key is in the cache, and if it's not in the cache, runs the dbCallback function that gets a response from the DB, saves it to the cache using the supplied key, and returns it
 *
 * @param {string} key // string key to check the cache and/or save the response to the cache
 * @param {number} time // numeric time value for the response to be stored in the cache
 * @param {Function<T>} dbCallback // the callback function to save a response to the cache, usually the function that gets the response to be cached from the DB
 *
 * @returns {T} // returns the response from the cache or DB
 *
 * @example
 * const cacheKeyPrefix = getCacheKeyPrefix(); // dnapicache3 or dnstagingapicache, or something like that
 * const { data: cities, error } = await checkCache(`${cacheKeyPrefix}::cities`, 86400 * 7, (id) => getCitiesFromDB(id));
*/
const checkCache = async <T>(key: string, time: number, dbCallback: () => Promise<T>): Promise<T> => {
  try {
    const cacheResponse = await redis.get(key);

    if (cacheResponse) {
      console.log(`CACHE HIT - ${key}`);

      const data = JSON.parse(cacheResponse);

      return await success<typeof data>(data);
    } else {
      console.log(`CACHE MISS - ${key}`);

      const dbResponse = await dbCallback();

      if (dbResponse.error) {
        // very important for the error to return before the setEx operation, otherwise the errored value gets saved to the cache
        // TODO: This line causes problems with REDIS or the cache to crash the entire API and therefore the entire site.  NOT GOOD.  There must be a better way!
        return await failure<T>(dbResponse.error, 'redis/checkCache');
      }

      const dbResult = dbResponse.data;

      const setExResult = await redis.set(key, JSON.stringify(dbResult), "PX", time);
      console.log(setExResult);

      return await success<T>(dbResult as T);
    }
  } catch (error) {
    console.error(error);
    return await failure<T>(error, 'redis/checkCache');
  }
}

export {
  checkCache
}
