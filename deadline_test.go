package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetDeadlineFromTitle(t *testing.T) {
	t0, err := time.Parse(time.RFC3339, "2018-12-03T14:36:04+08:00")
	assert.Nil(t, err)

	t1, directive, err := getDeadlineFromTitle(t0, "#1 <09> title content")
	assert.Nil(t, err)
	assert.Equal(t, "<09>", directive)
	assert.Equal(t, "2018-12-09", formatDate(t1))

	t1, directive, err = getDeadlineFromTitle(t0, "#2  <11-09> title content")
	assert.Nil(t, err)
	assert.Equal(t, "<11-09>", directive)
	assert.Equal(t, "2018-11-09", formatDate(t1))

	t1, directive, err = getDeadlineFromTitle(t0, "#3 <2018-11-09> title content")
	assert.Nil(t, err)
	assert.Equal(t, "<2018-11-09>", directive)
	assert.Equal(t, "2018-11-09", formatDate(t1))

	t1, directive, err = getDeadlineFromTitle(t0, " #4  <z1> title content")
	assert.Nil(t, err)
	assert.Equal(t, "<z1>", directive)
	assert.Equal(t, "2018-12-03", formatDate(t1))

	t1, directive, err = getDeadlineFromTitle(t0, " #4  <z7> title content")
	assert.Nil(t, err)
	assert.Equal(t, "<z7>", directive)
	assert.Equal(t, "2018-12-09", formatDate(t1))

	t1, directive, err = getDeadlineFromTitle(t0, " #5  <xz1> title content")
	assert.Nil(t, err)
	assert.Equal(t, "<xz1>", directive)
	assert.Equal(t, "2018-12-10", formatDate(t1))

	t1, directive, err = getDeadlineFromTitle(t0, " #6  <周一> title content")
	assert.Nil(t, err)
	assert.Equal(t, "<周一>", directive)
	assert.Equal(t, "2018-12-03", formatDate(t1))

	t1, directive, err = getDeadlineFromTitle(t0, " #7  <下周一> title content")
	assert.Nil(t, err)
	assert.Equal(t, "<下周一>", directive)
	assert.Equal(t, "2018-12-10", formatDate(t1))
}
