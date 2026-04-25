/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

package service

// SetChannelScoreOverride sets or clears a manual score override for a channel.
// Pass nil to clear the override and restore automatic calculation.
func SetChannelScoreOverride(channelID int, score *float64) {
	s := defaultChannelSuccessRateSelector
	s.mu.Lock()
	defer s.mu.Unlock()
	if score == nil {
		delete(s.scoreOverrides, channelID)
	} else {
		s.scoreOverrides[channelID] = *score
	}
}
