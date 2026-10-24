import test from 'node:test';
import assert from 'node:assert/strict';
import { availabilityError, worksiteDate, dateOffset, readableDay, calendarRange, contactFormatError } from '../dapper-platform/apps/web/app/workspace/employee_helpers.ts';

test('calendar loads complete Monday-based month/week/day ranges',()=>{
 assert.deepEqual(calendarRange('2026-11-01','month'),{from:'2026-10-26',to:'2026-12-07',days:42});
 assert.deepEqual(calendarRange('2026-10-25','week'),{from:'2026-10-19',to:'2026-10-26',days:7});
 assert.deepEqual(calendarRange('2026-12-31','day'),{from:'2026-12-31',to:'2027-01-01',days:1});
});
test('walk-in requires contact syntax, never an OTP',()=>{
 assert.equal(contactFormatError('Customer Name','person@example.ca','+1 (416) 555-0123'),'');
 assert.notEqual(contactFormatError('Customer Name','broken@','+14165550123'),'');
 assert.notEqual(contactFormatError('Customer Name','person@example.ca','4165550123'),'');
 assert.notEqual(contactFormatError('Customer Name','person@example.ca','+1abc4165550123'),'');
});

test('worksite dates use the assigned timezone across midnight', () => {
 assert.equal(worksiteDate('2026-10-24T02:00:00Z', 'America/Toronto'), '2026-10-23');
 assert.equal(worksiteDate('2026-10-24T02:00:00Z', 'Asia/Kolkata'), '2026-10-24');
});
test('calendar date arithmetic is independent of DST and host timezone', () => {
 assert.equal(dateOffset('2026-11-01', 1), '2026-11-02');
 assert.equal(dateOffset('2026-01-01', -1), '2025-12-31');
 assert.match(readableDay('2026-10-26'), /Monday/);
});
test('empty availability is an explicit unavailable decision', () => {
 assert.notEqual(availabilityError([], false), '');
 assert.equal(availabilityError([], true), '');
});
test('actual form validation rejects overlaps and malformed or reversed times', () => {
 for (const intervals of [[{day:0,start:'',end:''}], [{day:0,start:'14:00',end:'09:00'}], [{day:0,start:'09:00',end:'12:00'},{day:0,start:'11:00',end:'14:00'}]]) assert.notEqual(availabilityError(intervals, false), '');
 assert.equal(availabilityError([{day:0,start:'09:00',end:'12:00'},{day:0,start:'12:00',end:'14:00'}], false), '');
});
