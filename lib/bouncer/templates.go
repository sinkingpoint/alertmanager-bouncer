package bouncer

import (
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders/mirror"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders/silenceshaveauthor"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders/silenceshaveticket"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders/silencesnotonweekends"
)

var deciderTemplates = map[string]deciders.Template{
	"AllSilencesHaveAuthor":        deciders.TemplateFunc(silenceshaveauthor.New),
	"Mirror":                       deciders.TemplateFunc(mirror.New),
	"SilencesDontExpireOnWeekends": deciders.TemplateFunc(silencesnotonweekends.New),
	"LongSilencesHaveTicket":       deciders.TemplateFunc(silenceshaveticket.New),
}

func GetDeciderTemplate(name string) (deciders.Template, bool) {
	template, ok := deciderTemplates[name]
	return template, ok
}
