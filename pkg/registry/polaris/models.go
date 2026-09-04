package polaris

const SUCCESS = 200000
const EXISTS = 400201

type ServiceCreateRequest struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// InstanceRegisterRequest 注册服务请求
type InstanceRegisterRequest struct {
	// 必选，服务名
	Service string `json:"service"`
	// 必选，命名空间
	Namespace string `json:"namespace"`
	// 必选，服务监听host，支持IPv4/IPv6地址或主机域名
	Host string `json:"host"`
	// 必选，服务实例监听port
	Port int `json:"port"`

	// 服务协议
	Protocol *string `json:"protocol"`
	// 服务权重，默认100，范围0-10000
	Weight *int `json:"weight"`
	// 实例优先级，默认为0，数值越小，优先级越高
	Priority *int `json:"priority"`
	// 实例提供服务版本号
	Version *string `json:"version"`
	// 用户自定义metadata信息
	Metadata map[string]string `json:"metadata"`
	// 该服务实例是否健康，默认健康
	Healthy *bool `json:"healthy"`
}

func (r *InstanceRegisterRequest) SetProtocol(proto string) {
	r.Protocol = &proto
}

func (r *InstanceRegisterRequest) SetWeight(weight int) {
	r.Weight = &weight
}

func (r *InstanceRegisterRequest) SetHealthy(healthy bool) {
	r.Healthy = &healthy
}

type ServiceCreateResponse struct {
	Code      int    `json:"code"`
	Info      string `json:"info"`
	Size      int    `json:"size"`
	Responses []struct {
		Code      int         `json:"code"`
		Info      string      `json:"info"`
		Client    interface{} `json:"client"`
		Namespace interface{} `json:"namespace"`
		Service   struct {
			Name                 string            `json:"name"`
			Namespace            string            `json:"namespace"`
			Metadata             map[string]string `json:"metadata"`
			Ports                interface{}       `json:"ports"`
			Business             interface{}       `json:"business"`
			Department           interface{}       `json:"department"`
			CmdbMod1             interface{}       `json:"cmdb_mod1"`
			CmdbMod2             interface{}       `json:"cmdb_mod2"`
			CmdbMod3             interface{}       `json:"cmdb_mod3"`
			Comment              interface{}       `json:"comment"`
			Owners               interface{}       `json:"owners"`
			Token                string            `json:"token"`
			Ctime                interface{}       `json:"ctime"`
			Mtime                interface{}       `json:"mtime"`
			Revision             interface{}       `json:"revision"`
			PlatformId           interface{}       `json:"platform_id"`
			TotalInstanceCount   interface{}       `json:"total_instance_count"`
			HealthyInstanceCount interface{}       `json:"healthy_instance_count"`
			UserIds              []interface{}     `json:"user_ids"`
			GroupIds             []interface{}     `json:"group_ids"`
			RemoveUserIds        []interface{}     `json:"remove_user_ids"`
			RemoveGroupIds       []interface{}     `json:"remove_group_ids"`
			Id                   string            `json:"id"`
			Editable             interface{}       `json:"editable"`
			ExportTo             []interface{}     `json:"export_to"`
		} `json:"service"`
		Instance           interface{} `json:"instance"`
		Routing            interface{} `json:"routing"`
		Alias              interface{} `json:"alias"`
		RateLimit          interface{} `json:"rateLimit"`
		CircuitBreaker     interface{} `json:"circuitBreaker"`
		ConfigRelease      interface{} `json:"configRelease"`
		User               interface{} `json:"user"`
		UserGroup          interface{} `json:"userGroup"`
		AuthStrategy       interface{} `json:"authStrategy"`
		Relation           interface{} `json:"relation"`
		LoginResponse      interface{} `json:"loginResponse"`
		ModifyAuthStrategy interface{} `json:"modifyAuthStrategy"`
		ModifyUserGroup    interface{} `json:"modifyUserGroup"`
		Resources          interface{} `json:"resources"`
		OptionSwitch       interface{} `json:"optionSwitch"`
		InstanceLabels     interface{} `json:"instanceLabels"`
		Data               interface{} `json:"data"`
		ServiceContract    interface{} `json:"serviceContract"`
	} `json:"responses"`
}

// InstanceRegisterResponse 注册服务应答
type InstanceRegisterResponse struct {
	Code      int    `json:"code"`
	Info      string `json:"info"`
	Size      int    `json:"size"`
	Responses []struct {
		Code      int         `json:"code"`
		Info      string      `json:"info"`
		Client    interface{} `json:"client"`
		Namespace interface{} `json:"namespace"`
		Service   interface{} `json:"service"`
		Instance  struct {
			Id                string            `json:"id"`
			Service           string            `json:"service"`
			Namespace         string            `json:"namespace"`
			VpcId             interface{}       `json:"vpc_id"`
			Host              string            `json:"host"`
			Port              int               `json:"port"`
			Protocol          *string           `json:"protocol"`
			Version           interface{}       `json:"version"`
			Priority          interface{}       `json:"priority"`
			Weight            interface{}       `json:"weight"`
			EnableHealthCheck interface{}       `json:"enableHealthCheck"`
			HealthCheck       interface{}       `json:"healthCheck"`
			Healthy           interface{}       `json:"healthy"`
			Isolate           interface{}       `json:"isolate"`
			Location          interface{}       `json:"location"`
			Metadata          map[string]string `json:"metadata"`
			LogicSet          interface{}       `json:"logic_set"`
			Ctime             interface{}       `json:"ctime"`
			Mtime             interface{}       `json:"mtime"`
			Revision          interface{}       `json:"revision"`
			ServiceToken      interface{}       `json:"service_token"`
		} `json:"instance"`
		Routing            interface{} `json:"routing"`
		Alias              interface{} `json:"alias"`
		RateLimit          interface{} `json:"rateLimit"`
		CircuitBreaker     interface{} `json:"circuitBreaker"`
		ConfigRelease      interface{} `json:"configRelease"`
		User               interface{} `json:"user"`
		UserGroup          interface{} `json:"userGroup"`
		AuthStrategy       interface{} `json:"authStrategy"`
		Relation           interface{} `json:"relation"`
		LoginResponse      interface{} `json:"loginResponse"`
		ModifyAuthStrategy interface{} `json:"modifyAuthStrategy"`
		ModifyUserGroup    interface{} `json:"modifyUserGroup"`
		Resources          interface{} `json:"resources"`
		OptionSwitch       interface{} `json:"optionSwitch"`
		InstanceLabels     interface{} `json:"instanceLabels"`
		Data               interface{} `json:"data"`
		ServiceContract    interface{} `json:"serviceContract"`
	} `json:"responses"`
}

type QueryInstanceResponse struct {
	Code       int           `json:"code"`
	Info       string        `json:"info"`
	Amount     int           `json:"amount"`
	Size       int           `json:"size"`
	Namespaces []interface{} `json:"namespaces"`
	Services   []interface{} `json:"services"`
	Instances  []struct {
		Id                string            `json:"id"`
		Service           string            `json:"service"`
		Namespace         string            `json:"namespace"`
		VpcId             interface{}       `json:"vpc_id"`
		Host              string            `json:"host"`
		Port              int               `json:"port"`
		Protocol          string            `json:"protocol"`
		Version           interface{}       `json:"version"`
		Priority          interface{}       `json:"priority"`
		Weight            int               `json:"weight"`
		EnableHealthCheck interface{}       `json:"enableHealthCheck"`
		HealthCheck       interface{}       `json:"healthCheck"`
		Healthy           bool              `json:"healthy"`
		Isolate           bool              `json:"isolate"`
		Location          interface{}       `json:"location"`
		Metadata          map[string]string `json:"metadata"`
		LogicSet          interface{}       `json:"logic_set"`
		Ctime             string            `json:"ctime"`
		Mtime             string            `json:"mtime"`
		Revision          string            `json:"revision"`
		ServiceToken      string            `json:"service_token"`
	} `json:"instances"`
	Routings           []interface{} `json:"routings"`
	Aliases            []interface{} `json:"aliases"`
	RateLimits         []interface{} `json:"rateLimits"`
	ConfigWithServices []interface{} `json:"configWithServices"`
	Users              []interface{} `json:"users"`
	UserGroups         []interface{} `json:"userGroups"`
	AuthStrategies     []interface{} `json:"authStrategies"`
	Clients            []interface{} `json:"clients"`
	Data               []interface{} `json:"data"`
	Summary            interface{}   `json:"summary"`
}

type DeregisterInstanceRequest struct {
	// 实例ID，如果不设置 service、namespace、host、port，则必须设置 ID 字段
	ID string `json:"id,omitempty"`
	// 实例IP
	Host string `json:"host,omitempty"`
	// 命名空间
	Namespace string `json:"namespace,omitempty"`
	// 实例端口
	Port int64 `json:"port,omitempty"`
	// 服务名称
	Service string `json:"service,omitempty"`
}

type DeregisterInstanceResponse struct {
	Code      int    `json:"code"`
	Info      string `json:"info"`
	Size      int    `json:"size"`
	Responses []struct {
		Code      int         `json:"code"`
		Info      string      `json:"info"`
		Client    interface{} `json:"client"`
		Namespace interface{} `json:"namespace"`
		Service   interface{} `json:"service"`
		Instance  struct {
			Id                *string     `json:"id"`
			Service           string      `json:"service"`
			Namespace         string      `json:"namespace"`
			VpcId             interface{} `json:"vpc_id"`
			Host              string      `json:"host"`
			Port              int         `json:"port"`
			Protocol          interface{} `json:"protocol"`
			Version           interface{} `json:"version"`
			Priority          interface{} `json:"priority"`
			Weight            interface{} `json:"weight"`
			EnableHealthCheck interface{} `json:"enableHealthCheck"`
			HealthCheck       interface{} `json:"healthCheck"`
			Healthy           interface{} `json:"healthy"`
			Isolate           interface{} `json:"isolate"`
			Location          interface{} `json:"location"`
			Metadata          struct {
			} `json:"metadata"`
			LogicSet     interface{} `json:"logic_set"`
			Ctime        interface{} `json:"ctime"`
			Mtime        interface{} `json:"mtime"`
			Revision     interface{} `json:"revision"`
			ServiceToken interface{} `json:"service_token"`
		} `json:"instance"`
		Routing            interface{} `json:"routing"`
		Alias              interface{} `json:"alias"`
		RateLimit          interface{} `json:"rateLimit"`
		CircuitBreaker     interface{} `json:"circuitBreaker"`
		ConfigRelease      interface{} `json:"configRelease"`
		User               interface{} `json:"user"`
		UserGroup          interface{} `json:"userGroup"`
		AuthStrategy       interface{} `json:"authStrategy"`
		Relation           interface{} `json:"relation"`
		LoginResponse      interface{} `json:"loginResponse"`
		ModifyAuthStrategy interface{} `json:"modifyAuthStrategy"`
		ModifyUserGroup    interface{} `json:"modifyUserGroup"`
		Resources          interface{} `json:"resources"`
		OptionSwitch       interface{} `json:"optionSwitch"`
		InstanceLabels     interface{} `json:"instanceLabels"`
		Data               interface{} `json:"data"`
		ServiceContract    interface{} `json:"serviceContract"`
	} `json:"responses"`
}

type DeleteServiceResponse struct {
	Code      int    `json:"code"`
	Info      string `json:"info"`
	Size      int    `json:"size"`
	Responses []struct {
		Code      int         `json:"code"`
		Info      string      `json:"info"`
		Client    interface{} `json:"client"`
		Namespace interface{} `json:"namespace"`
		Service   struct {
			Name                 string            `json:"name"`
			Namespace            string            `json:"namespace"`
			Metadata             map[string]string `json:"metadata"`
			Ports                interface{}       `json:"ports"`
			Business             interface{}       `json:"business"`
			Department           interface{}       `json:"department"`
			CmdbMod1             interface{}       `json:"cmdb_mod1"`
			CmdbMod2             interface{}       `json:"cmdb_mod2"`
			CmdbMod3             interface{}       `json:"cmdb_mod3"`
			Comment              interface{}       `json:"comment"`
			Owners               interface{}       `json:"owners"`
			Token                interface{}       `json:"token"`
			Ctime                interface{}       `json:"ctime"`
			Mtime                interface{}       `json:"mtime"`
			Revision             interface{}       `json:"revision"`
			PlatformId           interface{}       `json:"platform_id"`
			TotalInstanceCount   interface{}       `json:"total_instance_count"`
			HealthyInstanceCount interface{}       `json:"healthy_instance_count"`
			UserIds              []interface{}     `json:"user_ids"`
			GroupIds             []interface{}     `json:"group_ids"`
			RemoveUserIds        []interface{}     `json:"remove_user_ids"`
			RemoveGroupIds       []interface{}     `json:"remove_group_ids"`
			Id                   interface{}       `json:"id"`
			Editable             interface{}       `json:"editable"`
			ExportTo             []interface{}     `json:"export_to"`
		} `json:"service"`
		Instance           interface{} `json:"instance"`
		Routing            interface{} `json:"routing"`
		Alias              interface{} `json:"alias"`
		RateLimit          interface{} `json:"rateLimit"`
		CircuitBreaker     interface{} `json:"circuitBreaker"`
		ConfigRelease      interface{} `json:"configRelease"`
		User               interface{} `json:"user"`
		UserGroup          interface{} `json:"userGroup"`
		AuthStrategy       interface{} `json:"authStrategy"`
		Relation           interface{} `json:"relation"`
		LoginResponse      interface{} `json:"loginResponse"`
		ModifyAuthStrategy interface{} `json:"modifyAuthStrategy"`
		ModifyUserGroup    interface{} `json:"modifyUserGroup"`
		Resources          interface{} `json:"resources"`
		OptionSwitch       interface{} `json:"optionSwitch"`
		InstanceLabels     interface{} `json:"instanceLabels"`
		Data               interface{} `json:"data"`
		ServiceContract    interface{} `json:"serviceContract"`
	} `json:"responses"`
}
