package database

func (d *XDICK) Get(key string, retvalue *string) bool {
	//d.logs.Debug("Get key='%s'", key)
	return d.get(key, retvalue)
}

func (d *XDICK) Set(key string, value string, overwrite bool) bool {
	//d.logs.Debug("Set key='%s' value='%s'", key, value)
	return d.set(key, value, overwrite)
}

func (d *XDICK) Del(key string) bool {
	//d.logs.Debug("Del key='%s'", key)
	return d.del(key)
}
