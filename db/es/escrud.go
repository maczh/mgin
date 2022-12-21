package es

import (
	"context"
	"errors"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/utils"
	"github.com/olivere/elastic"
	"strings"
)

func (e *ElasticSearch) AddDocument(database, table string, doc map[string]any, searchFields []string) (string, error) {
	indexName := fmt.Sprintf("%s_%s", database, table)
	if table == "" {
		indexName = database
	}
	if _, ok := doc["id"]; !ok {
		uuid, _ := uuid.NewV1()
		doc["id"] = uuid.String()
	} else {
		switch doc["id"].(type) {
		case float64, int64:
			doc["id"] = fmt.Sprintf("%v", doc["id"])
		}
	}
	if exists, _ := elastic.NewIndicesExistsService(e.Elastic).Index([]string{indexName}).Do(context.TODO()); !exists {
		//新建Index
		settings := buildIKPinyinSettings()
		mappings := buildMappings(doc, searchFields)
		settings["mappings"] = mappings
		logs.Debug("settings={}", settings)
		_, err := e.Elastic.CreateIndex(indexName).BodyJson(settings).Do(context.TODO())
		if err != nil {
			logs.Error("创建Index错误:{}", err.Error())
			return "", err
		}
	}
	resp, err := e.Elastic.Index().Index(indexName).Type("_doc").Id(doc["id"].(string)).BodyJson(doc).Do(context.TODO())
	logs.Debug("插入文档结果:{}", resp)
	if err != nil {
		return "", err
	}
	if resp.Result == "created" || resp.Result == "updated" {
		return doc["id"].(string), nil
	} else {
		return "", errors.New(resp.Result)
	}
}

func (e *ElasticSearch) AddDocuments(database, table string, docs []map[string]any, searchFields []string) ([]string, error) {
	indexName := fmt.Sprintf("%s_%s", database, table)
	if table == "" {
		indexName = database
	}
	for i, doc := range docs {
		if _, ok := doc["id"]; !ok {
			uid, _ := uuid.NewV4()
			docs[i]["id"] = uid.String()
			doc["id"] = uid.String()
		} else {
			switch doc["id"].(type) {
			case float64, int64:
				doc["id"] = fmt.Sprintf("%v", doc["id"])
			}
		}
	}
	if exists, _ := elastic.NewIndicesExistsService(e.Elastic).Index([]string{indexName}).Do(context.TODO()); !exists {
		//新建Index
		settings := buildIKPinyinSettings()
		mappings := buildMappings(docs[0], searchFields)
		settings["mappings"] = mappings
		logs.Debug("settings={}", settings)
		_, err := e.Elastic.CreateIndex(indexName).BodyJson(settings).Do(context.TODO())
		if err != nil {
			logs.Error("创建Index错误:{}", err.Error())
			return nil, err
		}
	}
	bulk := e.Elastic.Bulk()
	ids := make([]string, len(docs))
	for i, doc := range docs {
		ids[i] = doc["id"].(string)
		bulk.Add(elastic.NewBulkIndexRequest().Index(indexName).Id(doc["id"].(string)).Doc(doc))
	}
	resp, err := bulk.Do(context.Background())
	logs.Debug("批量插入返回结果:{}", resp)
	if err != nil {
		return nil, err
	}
	if resp.Errors {
		return nil, errors.New("批量入库存在错误")
	} else {
		return ids, nil
	}
}

func (e *ElasticSearch) DeleteDocument(database, table string, id string) (bool, error) {
	indexName := fmt.Sprintf("%s_%s", database, table)
	if table == "" {
		indexName = database
	}
	resp, err := e.Elastic.Delete().Index(indexName).Id(id).Do(context.TODO())
	if err != nil {
		logs.Error("删除文档错误:{}", err.Error())
		return false, err
	}
	if resp.Result == "deleted" {
		return true, nil
	} else {
		return false, errors.New(resp.Result)
	}
}

func (e *ElasticSearch) DeleteDocuments(database, table string, ids []string) (bool, error) {
	indexName := fmt.Sprintf("%s_%s", database, table)
	if table == "" {
		indexName = database
	}
	bulk := e.Elastic.Bulk()
	for _, id := range ids {
		bulk.Add(elastic.NewBulkDeleteRequest().Index(indexName).Id(id))
	}
	resp, err := bulk.Do(context.Background())
	logs.Debug("批量删除返回结果:{}", resp)
	if err != nil {
		return false, err
	}
	if resp.Errors {
		return false, errors.New("批量删除存在错误")
	} else {
		return true, nil
	}
}

func (e *ElasticSearch) UpdateDocument(database, table, id string, updateData map[string]any) (bool, error) {
	indexName := fmt.Sprintf("%s_%s", database, table)
	if table == "" {
		indexName = database
	}
	resp, err := e.Elastic.Update().Index(indexName).Id(id).Doc(updateData).Do(context.TODO())
	if err != nil {
		logs.Error("更新文档错误:{}", err.Error())
		return false, err
	}
	if resp.Result == "updated" {
		return true, nil
	} else {
		return false, errors.New(resp.Result)
	}
}

func (e *ElasticSearch) DeleteTable(database, table string) (bool, error) {
	indexName := fmt.Sprintf("%s_%s", database, table)
	if table == "" {
		indexName = database
	}
	resp, err := e.Elastic.DeleteIndex(indexName).Do(context.TODO())
	if err != nil {
		logs.Error("删除表错误:{}", err.Error())
		return false, err
	}
	if resp.Acknowledged {
		return true, nil
	} else {
		return false, errors.New("表删除错误")
	}
}

func (e *ElasticSearch) DeleteDatabase(database string) (bool, error) {
	indexName := fmt.Sprintf("%s_*", database)
	resp, err := e.Elastic.DeleteIndex(indexName).Do(context.TODO())
	if err != nil {
		logs.Error("删除数据库错误:{}", err.Error())
		return false, err
	}
	if resp.Acknowledged {
		return true, nil
	} else {
		return false, errors.New("数据库删除错误")
	}
}

func buildMappings(doc map[string]any, serachFields []string) map[string]any {
	mappings := make(map[string]any)
	properties := make(map[string]any)
	for k, v := range doc {
		if len(k) > 3 && utils.Right(k, 3) == "Jpy" {
			continue
		}
		fieldMapping := make(map[string]any)
		switch v.(type) {
		case string:
			fieldMapping["type"] = "text"
			subFieldMapping := make(map[string]any)
			subFieldMapping["keyword"] = map[string]any{"type": "keyword"}
			if utils.StringArrayContains(serachFields, k) {
				subFieldMapping["wildcard"] = map[string]any{
					"type":         "wildcard",
					"ignore_above": 102400,
				}
				//subFieldMapping["spy"] = map[string]any{
				//	"type":     "text",
				//	"analyzer": "pinyiSimpleIndexAnalyzer",
				//}
				//subFieldMapping["fpy"] = map[string]any{
				//	"type":     "text",
				//	"analyzer": "pinyiFullIndexAnalyzer",
				//}
				//subFieldMapping["iks"] = map[string]any{
				//	"type":     "text",
				//	"analyzer": "ikSmartIndexAnalyzer",
				//}
				//subFieldMapping["ikm"] = map[string]any{
				//	"type":     "text",
				//	"analyzer": "ikMaxIndexAnalyzer",
				//}
				//jpyMapping := map[string]any{
				//	"type": "text",
				//	"fields": map[string]any{
				//		"wildcard": map[string]any{
				//			"type":         "wildcard",
				//			"ignore_above": 102400,
				//		},
				//	},
				//}
				//properties[k+"Jpy"] = jpyMapping
			}
			fieldMapping["fields"] = subFieldMapping
		case float64:
			n := fmt.Sprintf("%v", v)
			if strings.Contains(n, ".") {
				fieldMapping["type"] = "double"
			} else {
				fieldMapping["type"] = "long"
			}
		case bool:
			fieldMapping["type"] = "boolean"
		case []any:
			vv := v.([]any)[0]
			switch vv.(type) {
			case string:
				fieldMapping["type"] = "text"
				subFieldMapping := make(map[string]any)
				subFieldMapping["keyword"] = map[string]any{"type": "keyword"}
				if utils.StringArrayContains(serachFields, k) {
					subFieldMapping["wildcard"] = map[string]any{
						"type":         "wildcard",
						"ignore_above": 102400,
					}
					//subFieldMapping["spy"] = map[string]any{
					//	"type":     "text",
					//	"analyzer": "pinyiSimpleIndexAnalyzer",
					//}
					//subFieldMapping["fpy"] = map[string]any{
					//	"type":     "text",
					//	"analyzer": "pinyiFullIndexAnalyzer",
					//}
					//subFieldMapping["iks"] = map[string]any{
					//	"type":     "text",
					//	"analyzer": "ikSmartIndexAnalyzer",
					//}
					//subFieldMapping["ikm"] = map[string]any{
					//	"type":     "text",
					//	"analyzer": "ikMaxIndexAnalyzer",
					//}
					//jpyMapping := map[string]any{
					//	"type": "text",
					//	"fields": map[string]any{
					//		"wildcard": map[string]any{
					//			"type":         "wildcard",
					//			"ignore_above": 102400,
					//		},
					//	},
					//}
					//properties[k+"Jpy"] = jpyMapping
				}
				fieldMapping["fields"] = subFieldMapping
			case float64:
				n := fmt.Sprintf("%v", v)
				if strings.Contains(n, ".") {
					fieldMapping["type"] = "double"
				} else {
					fieldMapping["type"] = "long"
				}
			case bool:
				fieldMapping["type"] = "boolean"
			case map[string]any:
				fieldMapping["type"] = "nested"
			default:
				fieldMapping["type"] = "text"
				subFieldMapping := make(map[string]any)
				subFieldMapping["keyword"] = map[string]any{"type": "keyword"}
				if utils.StringArrayContains(serachFields, k) {
					subFieldMapping["wildcard"] = map[string]any{
						"type":         "wildcard",
						"ignore_above": 102400,
					}
					//subFieldMapping["spy"] = map[string]any{
					//	"type":     "text",
					//	"analyzer": "pinyiSimpleIndexAnalyzer",
					//}
					//subFieldMapping["fpy"] = map[string]any{
					//	"type":     "text",
					//	"analyzer": "pinyiFullIndexAnalyzer",
					//}
					//subFieldMapping["iks"] = map[string]any{
					//	"type":     "text",
					//	"analyzer": "ikSmartIndexAnalyzer",
					//}
					//subFieldMapping["ikm"] = map[string]any{
					//	"type":     "text",
					//	"analyzer": "ikMaxIndexAnalyzer",
					//}
					//jpyMapping := map[string]any{
					//	"type": "text",
					//	"fields": map[string]any{
					//		"wildcard": map[string]any{
					//			"type":         "wildcard",
					//			"ignore_above": 102400,
					//		},
					//	},
					//}
					//properties[k+"Jpy"] = jpyMapping
				}
				fieldMapping["fields"] = subFieldMapping
			}
		case map[string]any:
			fieldMapping["type"] = "nested"
		default:
			fieldMapping["type"] = "text"
			subFieldMapping := make(map[string]any)
			subFieldMapping["keyword"] = map[string]any{"type": "keyword"}
			if utils.StringArrayContains(serachFields, k) {
				subFieldMapping["wildcard"] = map[string]any{
					"type":         "wildcard",
					"ignore_above": 102400,
				}
				//subFieldMapping["spy"] = map[string]any{
				//	"type":     "text",
				//	"analyzer": "pinyiSimpleIndexAnalyzer",
				//}
				//subFieldMapping["fpy"] = map[string]any{
				//	"type":     "text",
				//	"analyzer": "pinyiFullIndexAnalyzer",
				//}
				//subFieldMapping["iks"] = map[string]any{
				//	"type":     "text",
				//	"analyzer": "ikSmartIndexAnalyzer",
				//}
				//subFieldMapping["ikm"] = map[string]any{
				//	"type":     "text",
				//	"analyzer": "ikMaxIndexAnalyzer",
				//}
				//jpyMapping := map[string]any{
				//	"type": "text",
				//	"fields": map[string]any{
				//		"wildcard": map[string]any{
				//			"type":         "wildcard",
				//			"ignore_above": 102400,
				//		},
				//	},
				//}
				//properties[k+"Jpy"] = jpyMapping
			}
			fieldMapping["fields"] = subFieldMapping
		}
		if k == "id" {
			fieldMapping["type"] = "text"
			subFieldMapping := make(map[string]any)
			subFieldMapping["keyword"] = map[string]any{"type": "keyword"}
			fieldMapping["fields"] = subFieldMapping
		}
		properties[k] = fieldMapping
	}
	mappings["properties"] = properties
	return mappings
}

func buildIKPinyinSettings() map[string]any {
	//filterSpy := map[string]any{
	//	"type":                       "pinyin",
	//	"keep_first_letter":          true,
	//	"keep_separate_first_letter": false,
	//	"keep_full_pinyin":           false,
	//	"keep_joined_full_pinyin":    false,
	//	"keep_original":              false,
	//	"limit_first_letter_length":  10240,
	//	"lowercase":                  true,
	//}
	//filterFpy := map[string]any{
	//	"type":                         "pinyin",
	//	"keep_first_letter":            false,
	//	"keep_separate_first_letter":   false,
	//	"keep_full_pinyin":             true,
	//	"keep_joined_full_pinyin":      true,
	//	"none_chinese_pinyin_tokenize": true,
	//	"keep_original":                false,
	//	"limit_first_letter_length":    10240,
	//	"lowercase":                    true,
	//}
	filterNgram := map[string]any{
		"type":     "edge_ngram",
		"min_gram": 1,
		"max_gram": 50,
	}
	filters := map[string]any{
		//"pinyin_simple_filter": filterSpy,
		//"pinyin_full_filter":   filterFpy,
		"edge_ngram_filter": filterNgram,
	}
	//analyzerFpy := map[string]any{
	//	"filter": []string{
	//		"pinyin_full_filter",
	//		"lowercase",
	//	},
	//	"tokenizer": "keyword",
	//}
	//analyzerSpy := map[string]any{
	//	"filter": []string{
	//		"pinyin_simple_filter",
	//		"edge_ngram_filter",
	//		"lowercase",
	//	},
	//	"tokenizer": "keyword",
	//}
	//analyzerIKSmart := map[string]any{
	//	"type":      "custom",
	//	"tokenizer": "ik_smart",
	//}
	//analyzerIKMax := map[string]any{
	//	"type":      "custom",
	//	"tokenizer": "ik_max_word",
	//}
	analyzerNgram := map[string]any{
		"filter": []string{
			"edge_ngram_filter",
			"lowercase",
		},
		"type":      "custom",
		"tokenizer": "keyword",
	}
	analyzers := map[string]any{
		//"pinyiSimpleIndexAnalyzer": analyzerSpy,
		//"pinyiFullIndexAnalyzer":   analyzerFpy,
		//"ikSmartIndexAnalyzer":     analyzerIKSmart,
		//"ikMaxIndexAnalyzer":       analyzerIKMax,
		"ngramIndexAnalyzer": analyzerNgram,
	}
	analysis := map[string]any{
		"filter":   filters,
		"analyzer": analyzers,
	}
	settings := map[string]any{
		"analysis":           analysis,
		"refresh_interval":   "5s",
		"number_of_shards":   1,
		"number_of_replicas": 1,
	}
	return map[string]any{"settings": settings}
}
